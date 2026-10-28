package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.RequestResult
import dev.jasonpearson.krit.fir.handleRequestLine
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.AnalyzeOutcome
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * #740: with the planner on, a warm `analyzeWithCheck` answers the oracle from the facts its slice compiles
 * recorded instead of compiling the whole module again. Every answer must equal what a fresh whole-module oracle
 * compile gives, `cacheDeps` included (Go's oracle cache invalidates on those edges).
 */
class SharedOracleIncrementalTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val snaps get() = tmp.resolve("snapshots").toFile()
    private val root get() = tmp.resolve("repo/.krit/fir-incremental").toFile()
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun write(rel: String, text: String) = src.writeSource(rel, text)
    private fun paths() = kotlinFilesIn(src)

    private fun module() {
        write("Base.kt", "package p\n\nopen class Base {\n    open fun size(): Int = 1\n}\n\ninterface Named { val name: String }\n")
        write("Sub.kt", "package p\n\nclass Sub : Base(), Named {\n    override val name: String = \"s\"\n    override fun size(): Int = 2\n}\n")
        write("Use.kt", "package p\n\nfun use(s: Sub): Int {\n    val n = s.name.length\n    return s.size() + n\n}\n")
        write("Other.kt", "package p\n\nfun other(): String? = if (use(Sub()) > 1) \"x\" else null\n")
    }

    private fun session(planning: Boolean = true, kotlinArgs: List<String> = emptyList()): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), kotlinArgs = kotlinArgs, incrementalRecording = planning, incrementalPlanning = planning)
            .also { it.snapshotRootOverride = snaps; sessions += it }

    private fun request() = CheckRequest(id = 7, command = "check", files = paths().map { FileRef(it) }, rules = emptyList(), incrementalDir = root.path)

    private fun AnalysisSession.shared(): AnalyzeOutcome {
        val (outcome, checked) = analyzeWithCheck(paths(), request())
        assertNotNull(checked)
        return outcome
    }

    private fun fresh(kotlinArgs: List<String> = emptyList()): AnalyzeOutcome =
        session(planning = false, kotlinArgs = kotlinArgs).analyzeFull(paths())

    private fun assertSameAsFresh(got: AnalyzeOutcome, kotlinArgs: List<String> = emptyList()) {
        val want = fresh(kotlinArgs)
        assertEquals(want.result.files.keys.sorted(), got.result.files.keys.sorted())
        for ((path, payload) in want.result.files) assertEquals(payload, got.result.files[path], path)
        assertEquals(want.result.dependencies.toSortedMap(), got.result.dependencies.toSortedMap())
        assertEquals(want.cacheDeps.depPathsByFile.mapValues { it.value.toSortedSet() }.toSortedMap(),
            got.cacheDeps.depPathsByFile.mapValues { it.value.toSortedSet() }.toSortedMap())
        assertEquals(want.cacheDeps.perFileDeps.mapValues { it.value.toSortedMap() }.toSortedMap(),
            got.cacheDeps.perFileDeps.mapValues { it.value.toSortedMap() }.toSortedMap())
    }

    private fun p(name: String) = File(src, name).path

    @Test fun aBodyEditIsAnsweredFromTheSlice() {
        module()
        val s = session()
        assertSameAsFresh(s.shared())
        assertFalse(s.lastOracleFromCache, "the cold request compiles the module")

        write("Use.kt", "package p\n\nfun use(s: Sub): Int {\n    val n = s.name.length + 1\n    return s.size() + n\n}\n")
        val warm = s.shared()
        assertEquals("incremental", s.lastPlan?.path?.name?.lowercase())
        assertTrue(s.lastOracleFromCache, "the warm request is answered from the slice and the cached facts")
        assertSameAsFresh(warm)
    }

    @Test fun aSubclassCompiledAloneKeepsItsSupertypeEdges() {
        module()
        val s = session()
        s.shared()
        // Sub alone is compiled; Base is read from the snapshot as a binary, yet the edges must name Base.kt.
        write("Sub.kt", "package p\n\nclass Sub : Base(), Named {\n    override val name: String = \"t\"\n    override fun size(): Int = 3\n}\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache)
        assertEquals(setOf(p("Base.kt")), warm.cacheDeps.depPathsByFile[p("Sub.kt")])
        assertSameAsFresh(warm)
    }

    @Test fun aSupertypeSignatureChangeRefreshesItsDependents() {
        module()
        val s = session()
        s.shared()
        write("Base.kt", "package p\n\nopen class Base {\n    open fun size(): Long = 1\n}\n\ninterface Named { val name: String }\n")
        write("Sub.kt", "package p\n\nclass Sub : Base(), Named {\n    override val name: String = \"s\"\n    override fun size(): Long = 2\n}\n")
        val warm = s.shared()
        assertSameAsFresh(warm)
        write("Use.kt", "package p\n\nfun use(s: Sub): Long {\n    val n = s.name.length\n    return s.size() + n\n}\n")
        assertSameAsFresh(s.shared())
    }

    @Test fun aCalleeReadAsABinaryKeepsItsSourceRetentionAnnotations() {
        // Class files drop SOURCE-retention annotations; a caller compiled alone must still report them.
        write("Ann.kt", "package p\n\n@Target(AnnotationTarget.FUNCTION, AnnotationTarget.CONSTRUCTOR)\n" +
            "@Retention(AnnotationRetention.SOURCE)\nannotation class Marker\n\n" +
            "@Marker\nfun target(): Int = 1\n\n@Suppress(\"unused\")\n@Deprecated(\"x\")\nfun both(x: Int): Int = x\n\n" +
            "class Holder @Marker constructor(val v: Int) {\n    @Marker fun member(s: String): Int = s.length\n}\n")
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val a = target()\n    val b = both(1)\n    val h = Holder(2)\n    return a + b + h.member(\"a\")\n}\n")
        val s = session()
        assertSameAsFresh(s.shared())
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val a = target()\n    val b = both(2)\n    val h = Holder(3)\n    return a + b + h.member(\"bb\")\n}\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache, "the caller alone is compiled, against Ann.kt as a class file")
        val calls = warm.result.files.getValue(p("Caller.kt")).expressions.values
        assertTrue(calls.any { it.callTarget == "p.target" && "p.Marker" in it.annotations }, calls.toString())
        assertTrue(calls.any { it.callTarget == "p.both" && "kotlin.Suppress" in it.annotations }, calls.toString())
        assertTrue(calls.any { it.callTarget == "p.Holder.member" && "p.Marker" in it.annotations }, calls.toString())
        assertSameAsFresh(warm)
    }

    @Test fun overloadsThatDifferOnlyInContextParametersKeepTheirOwnAnnotations() {
        val args = listOf("-Xcontext-parameters")
        write("Ann.kt", "package p\n\n@Retention(AnnotationRetention.SOURCE)\nannotation class First\n\n" +
            "@Retention(AnnotationRetention.SOURCE)\nannotation class Second\n\nclass A\nclass B\n\n" +
            "@First\ncontext(a: A) fun target(): Int = 1\n\n@Second\ncontext(b: B) fun target(): Int = 2\n")
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val x = with(B()) { target() }\n    return x\n}\n")
        val s = session(kotlinArgs = args)
        assertSameAsFresh(s.shared(), args)
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val x = with(B()) { target() }\n    return x + 1\n}\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache)
        val call = warm.result.files.getValue(p("Caller.kt")).expressions.values.single { it.callTarget == "p.target" }
        assertEquals(listOf("p.Second"), call.annotations)
        assertSameAsFresh(warm, args)
    }

    @Test fun genericOverloadsThatDifferOnlyInBoundsKeepTheirOwnAnnotations() {
        write("Ann.kt", "package p\n\n@Retention(AnnotationRetention.SOURCE)\nannotation class Numeric\n\n" +
            "@Numeric\nfun <T : Number> target(x: T): Int = 1\n\nfun <T : CharSequence> target(x: T): Int = 2\n")
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val a = target(1)\n    val b = target(\"s\")\n    return a + b\n}\n")
        val s = session()
        assertSameAsFresh(s.shared())
        write("Caller.kt", "package p\n\nfun caller(): Int {\n    val a = target(2)\n    val b = target(\"t\")\n    return a + b\n}\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache)
        val calls = warm.result.files.getValue(p("Caller.kt")).expressions.values.filter { it.callTarget == "p.target" }
        assertEquals(listOf(listOf("p.Numeric"), emptyList()), calls.sortedBy { it.startByte }.map { it.annotations })
        assertSameAsFresh(warm)
    }

    @Test fun aPrivateMemberEditInASupertypeRefreshesTheSubclassEdges() {
        // A private member is not part of Base's surface, so Sub is not compiled again, yet the projection of Base
        // that Sub's edges carry lists it.
        write("Base.kt", "package p\n\nopen class Base {\n    open fun size(): Int = helper()\n    private fun helper(): Int = 1\n}\n\ninterface Named { val name: String }\n")
        write("Sub.kt", "package p\n\nclass Sub : Base(), Named {\n    override val name: String = \"s\"\n}\n")
        val s = session()
        assertSameAsFresh(s.shared())
        write("Base.kt", "package p\n\nopen class Base {\n    open fun size(): Int = helper().toInt()\n    private fun helper(): Long = 1\n}\n\ninterface Named { val name: String }\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache)
        assertSameAsFresh(warm)
    }

    @Test fun addedAndDeletedFilesFollow() {
        module()
        val s = session()
        s.shared()
        write("Extra.kt", "package p\n\nclass Extra : Base() {\n    fun twice(): Int = size() * 2\n}\n")
        assertSameAsFresh(s.shared())
        File(src, "Extra.kt").delete()
        File(src, "Other.kt").delete()
        assertSameAsFresh(s.shared())
    }

    @Test fun aCompileErrorAndItsRepairMatchTheFreshDiagnostics() {
        module()
        val s = session()
        s.shared()
        write("Use.kt", "package p\n\nfun use(s: Sub): Int {\n    val n = s.missing\n    return s.size() + n\n}\n")
        assertSameAsFresh(s.shared())
        write("Use.kt", "package p\n\nfun use(s: Sub): Int {\n    val n = s.name.length\n    return s.size() + n\n}\n")
        assertSameAsFresh(s.shared())
    }

    @Test fun aCheckOnlySliceLeavesTheFactsBehindUntilTheOracleCompilesAgain() {
        module()
        val s = session()
        s.shared()
        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size()\n")
        // A plain check slices without the oracle: its facts are now a generation behind.
        s.check(1, paths().map { FileRef(it) }, emptySet())
        val refilled = s.shared()
        assertFalse(s.lastOracleFromCache, "facts a check-only slice left behind are not used")
        assertSameAsFresh(refilled)

        write("Other.kt", "package p\n\nfun other(): String = use(Sub()).toString()\n")
        val warm = s.shared()
        assertTrue(s.lastOracleFromCache, "the oracle compile that followed refilled them")
        assertSameAsFresh(warm)
    }

    @Test fun aRestartedProcessServesTheOracleFromThePersistedFacts() {
        module()
        session().apply { shared(); flushPersistence() }.dispose()

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 2\n")
        val restarted = session()
        val first = restarted.shared()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "load: ${restarted.persistence!!.lastLoad}")
        assertTrue(restarted.lastOracleFromCache, "the bundle carries the oracle facts: no whole-module oracle compile")
        assertSameAsFresh(first)

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 3\n")
        val warm = restarted.shared()
        assertTrue(restarted.lastOracleFromCache)
        assertSameAsFresh(warm)
    }

    @Test fun aRestartAfterADeletedFileStillServesFromThePersistedFacts() {
        module()
        val first = session()
        first.shared()
        // Deleted and handled incrementally: the persisted facts must not keep naming it.
        File(src, "Other.kt").delete()
        assertSameAsFresh(first.shared())
        assertTrue(first.lastOracleFromCache)
        first.flushPersistence()
        first.dispose()

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 5\n")
        val restarted = session()
        val got = restarted.shared()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "load: ${restarted.persistence!!.lastLoad}")
        assertTrue(restarted.lastOracleFromCache, "the facts persisted after the deletion are used")
        assertSameAsFresh(got)
    }

    @Test fun oracleFactsThatNameAFileOutsideTheModuleAreLeftOutOfTheLoad() {
        module()
        val first = session()
        first.shared()
        first.flushPersistence()
        val key = first.compileKeyNow()
        first.dispose()
        // Tamper with the persisted facts only: one entry now names a file the state does not record.
        val store = BundleStore(root)
        val file = store.bundleFile(key)
        val bundle = BundleCodec.decode(file.readBytes(), key)
        val oracle = bundle.oracle!!
        val entries = LinkedHashMap(oracle.entries).apply { put("/elsewhere/Evil.kt", values.first()) }
        file.writeBytes(BundleCodec.encode(key, bundle.snapshotId, bundle.state, bundle.verdicts, bundle.verdictGeneration,
            PersistedOracle(oracle.generation, oracle.order, entries)))

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 4\n")
        val restarted = session()
        val got = restarted.shared()
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "the state still loads: ${restarted.persistence!!.lastLoad}")
        assertFalse(restarted.lastOracleFromCache, "the tampered facts are not used: the oracle compiles and refills")
        assertSameAsFresh(got)
    }

    @Test fun anOracleOnlyRequestOnAnUnchangedModuleIsServedFromTheFacts() {
        module()
        val s = session()
        s.shared()
        // #957: the check is served by its own cache, so only the oracle is asked. Nothing changed: no compile.
        val unchanged = s.analyzeFull(paths())
        assertTrue(s.lastOracleFromCache, "an unchanged module is answered from the facts")
        assertSameAsFresh(unchanged)

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 6\n")
        val edited = s.analyzeFull(paths())
        assertFalse(s.lastOracleFromCache, "an edit the state has not seen compiles")
        assertSameAsFresh(edited)

        write("Extra.kt", "package p\n\nfun extra(): Int = 1\n")
        val added = s.analyzeFull(paths())
        assertFalse(s.lastOracleFromCache, "an added file compiles")
        assertSameAsFresh(added)
    }

    @Test fun aSessionWithoutStateAnswersARepeatedOracleOnlyRequestFromItsLastCompile() {
        module()
        // No check ever ran here (a restarted daemon whose check the Go cache serves): there is no incremental state.
        val s = session()
        assertSameAsFresh(s.analyzeFull(paths()))
        assertFalse(s.lastOracleFromCache)
        val again = s.analyzeFull(paths())
        assertTrue(s.lastOracleFromCache, "nothing changed since the last plain compile")
        assertSameAsFresh(again)

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 7\n")
        val edited = s.analyzeFull(paths())
        assertFalse(s.lastOracleFromCache, "an edit compiles")
        assertSameAsFresh(edited)
        assertSameAsFresh(s.analyzeFull(paths()))
        assertTrue(s.lastOracleFromCache)

        File(src, "Other.kt").delete()
        assertSameAsFresh(s.analyzeFull(paths()))
        assertFalse(s.lastOracleFromCache, "a deleted file compiles")
    }

    @Test fun anOracleOnlyRequestWithoutPlanningCompiles() {
        module()
        val s = session(planning = false)
        s.analyzeFull(paths())
        s.analyzeFull(paths())
        assertFalse(s.lastOracleFromCache)
    }

    @Test fun aRestartedProcessAnswersAnOracleOnlyRequestFromTheBundleItNames() {
        module()
        session().apply { shared(); flushPersistence() }.dispose()

        val unnamed = session()
        unnamed.analyzeFull(paths())
        assertFalse(unnamed.lastOracleFromCache, "a session that was never told where the bundle is compiles")

        val restarted = session()
        val line = """{"id":3,"command":"analyzeAll","incrementalDir":"${root.path}"}"""
        val response = handleRequestLine(line, restarted, 0)
        assertTrue(response is RequestResult.Response && "\"result\"" in response.json, "response: $response")
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "load: ${restarted.persistence!!.lastLoad}")
        assertTrue(restarted.lastOracleFromCache, "the analyzeAll request named the bundle: no compile")
        assertSameAsFresh(restarted.analyzeFull(paths()))
        assertTrue(restarted.lastOracleFromCache)
    }
}
