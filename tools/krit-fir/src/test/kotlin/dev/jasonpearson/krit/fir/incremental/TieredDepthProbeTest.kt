package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import java.io.File
import java.nio.file.Files
import kotlin.test.assertTrue

/**
 * Research probe for the tiered rules (#740, tiered-rules-findings.md, option A): for a chain of declarations spread
 * over files (A uses, B forwards, C holds the fact), how many files beyond A must be in source for each tiered rule to
 * give A the verdict of a full compile, does the compiler's LookupTracker closure of A reach those files, and does a
 * body-only edit of the last file (no surface change) reach A through (a) the step-4 dependents and (b) the reverse
 * lookup closure.
 *
 * Tagged: run with `./gradlew tieredResearchTest -Pkrit.fir.tiered.mode=depth`.
 */
@Tag(TieredResearch.TAG)
class TieredDepthProbeTest {

    private class Chain(
        val rule: String,
        /** A (the use) first, then the declarations it reaches, in chain order. */
        val files: List<Pair<String, String>>,
        /** A body-only edit of one chain file that flips A's full-compile verdict, or null. */
        val bodyEdit: Pair<String, String>? = null,
        val config: Map<String, Map<String, Any?>> = emptyMap(),
    )

    private val chains = listOf(
        Chain(
            "HardcodedSecretKey",
            listOf(
                "A.kt" to "package p\nimport javax.crypto.spec.SecretKeySpec\nfun key() = SecretKeySpec(java.util.Base64.getDecoder().decode(B.K1), \"AES\")\n",
                "B.kt" to "package p\nobject B { val K1: String = C.K2 }\n",
                "C.kt" to "package p\nobject C { val K2: String = D.K3 }\n",
                "D.kt" to "package p\nobject D { val K3: String = \"c2VjcmV0a2V5MTIzNDU2Nw==\" }\n",
            ),
            bodyEdit = "D.kt" to "package p\nobject D { val K3: String = System.getenv(\"K\").orEmpty() }\n",
        ),
        Chain(
            "StaticIv",
            listOf(
                "A.kt" to "package p\nimport javax.crypto.spec.IvParameterSpec\nfun iv() = IvParameterSpec(\"\${B.P}\".toByteArray())\n",
                "B.kt" to "package p\nobject B { val P: String = \"\${C.Q}\" }\n",
                "C.kt" to "package p\nobject C { val Q: String = \"0123456789abcdef\" }\n",
            ),
            bodyEdit = "C.kt" to "package p\nobject C { val Q: String = System.getenv(\"IV\").orEmpty() }\n",
        ),
        Chain(
            "WeakKeySize",
            listOf(
                "A.kt" to "package p\nfun weak() { gen.init(64) }\n",
                "B.kt" to "package p\nimport javax.crypto.KeyGenerator\nval gen: KeyGenerator = KeyGenerator.getInstance(\"AES\")\n",
            ),
            bodyEdit = "B.kt" to "package p\nimport javax.crypto.KeyGenerator\nval gen: KeyGenerator = KeyGenerator.getInstance(\"Blowfish\")\n",
        ),
        Chain(
            "LongLogTag",
            listOf(
                "A.kt" to "package p\nimport android.util.Log\nfun log() { Log.d(B.TAG, \"m\") }\n",
                "B.kt" to "package p\nobject B { val TAG: String = C.TAG }\n",
                "C.kt" to "package p\nobject C { val TAG: String = \"AVeryLongTagThatExceedsTheLimit\" }\n",
            ),
            bodyEdit = "C.kt" to "package p\nobject C { val TAG: String = \"Short\" }\n",
        ),
        // A const tag survives in the binary, but its literal has no source there: does the message's spelling of an
        // escape sequence stay the same on a slice?
        Chain(
            "LongLogTag",
            listOf(
                "A.kt" to "package p\nimport android.util.Log\nfun log() { Log.d(Tags.T, \"m\") }\n",
                "B.kt" to "package p\nobject Tags { const val T = \"Tag\\u0041VeryLongTagThatExceedsTheLimit\" }\n",
            ),
        ),
        Chain(
            "LocaleGetDefaultForFormatting",
            listOf(
                "A.kt" to "package p\nimport java.util.Locale\nfun fmt() = F1.withLocale(Locale.getDefault())\n",
                "B.kt" to "package p\nimport java.time.format.DateTimeFormatter\nval F1: DateTimeFormatter = F0\n",
                "C.kt" to "package p\nimport java.time.format.DateTimeFormatter\nval F0: DateTimeFormatter = DateTimeFormatter.ISO_DATE\n",
            ),
            bodyEdit = "C.kt" to "package p\nimport java.time.format.DateTimeFormatter\nval F0: DateTimeFormatter = DateTimeFormatter.ofPattern(\"yyyy\")\n",
        ),
        Chain(
            "JdbcStatementExecute",
            listOf(
                "A.kt" to "package p\nimport java.sql.Statement\nfun run(s: Statement) { s.execute(B.query) }\n",
                "B.kt" to "package p\nobject B { val query: String = C.base }\n",
                "C.kt" to "package p\nobject C { val base: String = \"SELECT 1\" }\n",
            ),
            bodyEdit = "C.kt" to "package p\nobject C { val base: String = \"SELECT \" + System.getenv(\"T\") }\n",
        ),
        Chain(
            "CollectionsSynchronizedListIteration",
            listOf(
                "A.kt" to "package p\nfun dump() { for (x in items) println(x) }\n",
                "B.kt" to "package p\nimport java.util.Collections\nval items: MutableList<String> = Collections.synchronizedList(ArrayList())\n",
            ),
            bodyEdit = "B.kt" to "package p\nimport java.util.Collections\nval items: MutableList<String> = ArrayList()\n",
        ),
        Chain(
            "SharedFlowWithoutReplay",
            listOf(
                "A.kt" to "package p\nclass Vm { val events = factory1<Int>() }\n",
                "B.kt" to "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\nfun <T> factory1(): MutableSharedFlow<T> = factory2()\n",
                "C.kt" to "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\nfun <T> factory2(): MutableSharedFlow<T> = MutableSharedFlow()\n",
            ),
            bodyEdit = "C.kt" to "package p\nimport kotlinx.coroutines.flow.MutableSharedFlow\nfun <T> factory2(): MutableSharedFlow<T> = MutableSharedFlow(replay = 1)\n",
        ),
        Chain(
            "ShortAlarm",
            listOf(
                "A.kt" to "package p\nimport android.app.PendingIntent\nfun schedule(pi: PendingIntent) { Sched.setRepeating(0, 0L, 5000L, pi) }\n",
                "B.kt" to "package p\nimport android.app.PendingIntent\nobject Sched { fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { Inner.setRepeating(t, at, every, pi) } }\n",
                "C.kt" to "package p\nimport android.app.AlarmManager\nimport android.app.PendingIntent\nobject Inner {\n  private fun am(): AlarmManager = TODO()\n  fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { println(t) }\n}\n",
            ),
            bodyEdit = "C.kt" to "package p\nimport android.app.AlarmManager\nimport android.app.PendingIntent\nobject Inner {\n  private fun am(): AlarmManager = TODO()\n  fun setRepeating(t: Int, at: Long, every: Long, pi: PendingIntent) { am().setRepeating(t, at, every, pi) }\n}\n",
        ),
        Chain(
            "WorldWriteableFiles",
            listOf(
                "A.kt" to "package p\nfun mode() = B.MODE_WORLD_WRITEABLE\n",
                "B.kt" to "package p\nobject B { val MODE_WORLD_WRITEABLE: Int = C.MODE }\n",
                "C.kt" to "package p\nobject C { val MODE: Int = 0 }\n",
            ),
            bodyEdit = "C.kt" to "package p\nobject C { val MODE: Int = 2 }\n",
        ),
        Chain(
            "SecureRandom",
            listOf(
                "A.kt" to "package p\nfun seed() { rng.setSeed(1L) }\n",
                "B.kt" to "package p\nimport java.security.SecureRandom\nimport java.util.Random\nval rng: Random = SecureRandom()\n",
            ),
            bodyEdit = "B.kt" to "package p\nimport java.util.Random\nval rng: Random = Random()\n",
        ),
        Chain(
            "ShowToast",
            listOf(
                "A.kt" to "package p\nimport android.widget.Toast\nimport android.content.Context\nfun g(c: Context) {\n    saved = Toast.makeText(c, \"y\", 0)\n    saved?.show()\n}\n",
                "B.kt" to "package p\nimport android.widget.Toast\nimport android.content.Context\nlateinit var ctx: Context\nvar saved: Toast? = Toast.makeText(ctx, \"x\", 0)\n",
            ),
            bodyEdit = "B.kt" to "package p\nimport android.widget.Toast\nimport android.content.Context\nlateinit var ctx: Context\nvar saved: Toast? = null\n",
        ),
        Chain(
            "GetSignatures",
            listOf(
                "A.kt" to "package p\nimport android.content.pm.PackageManager\nfun f(pm: PackageManager) { pm.getPackageInfo(\"x\", FLAGS) }\n",
                "B.kt" to "package p\nval FLAGS = SIG\n",
                "C.kt" to "package p\nimport android.content.pm.PackageManager\nval SIG = PackageManager.GET_SIGNATURES\n",
            ),
            bodyEdit = "C.kt" to "package p\nimport android.content.pm.PackageManager\nval SIG = PackageManager.GET_META_DATA\n",
        ),
        Chain(
            "GlobalScopeLaunchInViewModel",
            listOf(
                "A.kt" to "package p\nimport kotlinx.coroutines.launch\nclass FooViewModel { fun f() { appScope.launch { } } }\n",
                "B.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nval appScope: CoroutineScope = rootScope\n",
                "C.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.GlobalScope\nval rootScope: CoroutineScope = GlobalScope\n",
            ),
            bodyEdit = "C.kt" to "package p\nimport kotlinx.coroutines.CoroutineScope\nimport kotlinx.coroutines.MainScope\nval rootScope: CoroutineScope = MainScope()\n",
        ),
        Chain(
            "TooGenericExceptionThrown",
            listOf(
                "A.kt" to "package p\nfun f() { throw CustomException() }\n",
                "B.kt" to "package p\nclass CustomException : RuntimeException()\n",
            ),
            config = mapOf(
                "TooGenericExceptionThrown" to mapOf(
                    "exceptionNames" to listOf("Exception", "Throwable", "Error", "RuntimeException", "CustomException"),
                ),
            ),
        ),
        Chain(
            "SerialVersionUIDInSerializableClass",
            listOf(
                "A.kt" to "package app\nclass FatalFailure(msg: String) : kotlin.Error(msg)\n",
                "B.kt" to "package model\nsealed class Resource : java.io.Serializable {\n  companion object { private const val serialVersionUID = 1L }\n  data class Error(val message: String) : Resource()\n}\n",
            ),
            bodyEdit = "B.kt" to "package model\nsealed class Resource {\n  data class Error(val message: String) : Resource()\n}\n",
        ),
    )

    private class Row(
        val rule: String,
        val chainFiles: Int,
        /** Smallest k such that A + the first k chain files in source gives A its full verdict (-1: none). */
        val minPrefix: Int,
        val perPrefix: List<Boolean>,
        /** Lookup-closure hop distance of each chain file from A (-1: not reachable). */
        val hops: List<Int>,
        val closureSliceCorrect: Boolean,
        val closureSize: Int,
        val fullDiffersOnBodyEdit: Boolean?,
        val surfaceChangedOnBodyEdit: Boolean?,
        val step4DependentsReachA: Boolean?,
        val reverseClosureReachesA: Boolean?,
        val expandedSliceCorrectAfterEdit: Boolean?,
        val note: String,
    )

    @Test
    fun depthPerRule() {
        val only = System.getProperty("krit.fir.tiered.only")?.takeIf { it.isNotBlank() }
        val work = Files.createTempDirectory("krit-fir-tiered-depth").toFile()
        val rows = ArrayList<Row>()
        try {
            val cp = listOf(CompilerTestStubs.compile(work).path)
            for (chain in chains) {
                if (only != null && !chain.rule.contains(only)) continue
                rows += probe(chain, cp, File(work, chain.rule))
            }
        } finally {
            work.deleteRecursively()
        }
        println("## tiered depth probe")
        println("| rule | chain files | A's full verdict needs A + first k chain files | per prefix (k=0..) | lookup hops of chain files | slice = A + lookup closure correct (size) | body edit: full changes / surface changed / step-4 dependents reach A / reverse closure reaches A / expanded slice correct |")
        println("|---|---|---|---|---|---|---|")
        for (r in rows) {
            println(
                "| ${r.rule} | ${r.chainFiles} | ${if (r.minPrefix < 0) "never" else r.minPrefix.toString()} | ${r.perPrefix.joinToString("/") { if (it) "=" else "≠" }} | " +
                    "${r.hops} | ${r.closureSliceCorrect} (${r.closureSize}) | ${r.fullDiffersOnBodyEdit} / ${r.surfaceChangedOnBodyEdit} / ${r.step4DependentsReachA} / ${r.reverseClosureReachesA} / ${r.expandedSliceCorrectAfterEdit} | ${r.note}",
            )
        }
        assertTrue(rows.isNotEmpty(), "no chain ran")
    }

    private fun probe(chain: Chain, cp: List<String>, root: File): Row {
        root.deleteRecursively()
        val src = File(root, "src").also { it.mkdirs() }
        val paths = chain.files.map { (name, text) -> canon(src.writeSource(name, text).path) }
        val a = paths.first()
        val all = paths.map { FileRef(it) }
        fun ruleFindings(r: dev.jasonpearson.krit.fir.runner.BatchResult, path: String) =
            r.verdictOf(path).findings.filter { it.startsWith(chain.rule + "@") }

        val ref = CompilerTestStubs.session(listOf(src.path), cp, record = false).check(1, all, emptySet(), chain.config)
        val expected = ruleFindings(ref, a)
        val session = CompilerTestStubs.session(listOf(src.path), cp, record = true, snapshots = File(root, "snapshots"))
        try {
            val full = session.check(1, all, emptySet(), chain.config)
            check(ruleFindings(full, a) == expected) { "recording check differs from the plain check" }
            val state = session.incrementalState
            if (state.snapshot == null) {
                return Row(chain.rule, paths.size - 1, -1, emptyList(), emptyList(), false, 0, null, null, null, null, null,
                    "no snapshot: ${state.fallbackReason()} ${ref.errorFiles.values.firstOrNull()?.take(120)}")
            }
            val closure = LookupClosure(state)
            val hops = closure.hops(a)
            val chainHops = paths.drop(1).map { hops[it] ?: -1 }

            val perPrefix = ArrayList<Boolean>()
            for (k in 0 until paths.size) {
                val slice = slice(session, paths.take(k + 1), chain.config)
                perPrefix += ruleFindings(slice, a) == expected
            }
            val minPrefix = perPrefix.indexOfFirst { it }
            val closureSet = listOf(a) + closure.closure(listOf(a), Int.MAX_VALUE)
            val closureCorrect = ruleFindings(slice(session, closureSet, chain.config), a) == expected

            val note = "full: ${expected.size} finding(s)"
            val edit = chain.bodyEdit ?: return Row(chain.rule, paths.size - 1, minPrefix, perPrefix, chainHops, closureCorrect,
                closureSet.size - 1, null, null, null, null, null, note)

            // Body-only edit of a chain file.
            val edited = canon(File(src, edit.first).path)
            File(edited).writeText(edit.second)
            val ref2 = CompilerTestStubs.session(listOf(src.path), cp, record = false).check(2, all, emptySet(), chain.config)
            val expected2 = ruleFindings(ref2, a)
            val sliceEdited = session.sliceCheck(3, listOf(FileRef(edited)), emptySet(), chain.config)
            check(sliceEdited is SliceOutcome.Checked) { "slice of the edited file fell back: $sliceEdited" }
            val surfaceChanged = state.surfaceChanged(edited)
            val deps = state.dependentsOf(listOf(edited))
            val closure2 = LookupClosure(state)
            val reverse = closure2.affected(listOf(edited), Int.MAX_VALUE)
            val expandedSet = (listOf(a) + closure2.closure(listOf(a), Int.MAX_VALUE)).distinct()
            val expandedCorrect = ruleFindings(slice(session, expandedSet, chain.config), a) == expected2
            return Row(
                chain.rule, paths.size - 1, minPrefix, perPrefix, chainHops, closureCorrect, closureSet.size - 1,
                expected2 != expected, surfaceChanged, a in deps, a in reverse, expandedCorrect,
                "$note; after edit ${expected2.size}",
            )
        } finally {
            session.dispose()
        }
    }

    private fun slice(session: dev.jasonpearson.krit.fir.runner.AnalysisSession, files: List<String>, config: Map<String, Map<String, Any?>>) =
        when (val o = session.sliceCheck(2, files.map { FileRef(it) }, emptySet(), config)) {
            is SliceOutcome.Checked -> o.result
            is SliceOutcome.Fallback -> error("slice fell back: ${o.reason} ${o.cause}")
        }
}
