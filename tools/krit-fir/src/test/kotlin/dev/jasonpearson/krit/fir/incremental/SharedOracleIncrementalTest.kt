package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.CheckRequest
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.AnalyzeOutcome
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

    private fun session(planning: Boolean = true): AnalysisSession =
        AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = planning, incrementalPlanning = planning)
            .also { it.snapshotRootOverride = snaps; sessions += it }

    private fun request() = CheckRequest(id = 7, command = "check", files = paths().map { FileRef(it) }, rules = emptyList(), incrementalDir = root.path)

    private fun AnalysisSession.shared(): AnalyzeOutcome {
        val (outcome, checked) = analyzeWithCheck(paths(), request())
        assertNotNull(checked)
        return outcome
    }

    private fun fresh(): AnalyzeOutcome = plainSession(src).also { sessions += it }.analyzeFull(paths())

    private fun assertSameAsFresh(got: AnalyzeOutcome) {
        val want = fresh()
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

    @Test fun aRestartedProcessRefillsOnceThenServesFromSlices() {
        module()
        session().apply { shared(); flushPersistence() }.dispose()

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 2\n")
        val restarted = session()
        assertSameAsFresh(restarted.shared())
        assertFalse(restarted.lastOracleFromCache, "facts are not persisted: the first warm request compiles the oracle")

        write("Use.kt", "package p\n\nfun use(s: Sub): Int = s.size() + 3\n")
        val warm = restarted.shared()
        assertTrue(restarted.lastOracleFromCache)
        assertSameAsFresh(warm)
    }
}
