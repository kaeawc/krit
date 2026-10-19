@file:OptIn(
    org.jetbrains.kotlin.CoreEnvironmentDeprecation::class,
    org.jetbrains.kotlin.config.CompilerConfiguration.Internals::class,
    org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class,
)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import com.intellij.openapi.util.Disposer
import dev.jasonpearson.krit.fir.FirRuleDiscovery
import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.psi.KtModifierListOwner
import org.jetbrains.kotlin.psi.KtPsiFactory
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import java.io.File
import java.nio.file.Files
import java.util.TreeMap
import kotlin.test.assertTrue

/**
 * Behavioural check of the rule tiering: which krit rules give a different verdict for a file when the rest of its
 * module comes from class files (an incremental slice compile against a binary snapshot) instead of source?
 *
 * Corpus: the compiler-test data (`compiler-tests/src/test/data/diagnostic`, one scenario per file, written against
 * the shared stubs, which are compiled once into a stub library). Each scenario with at least two top-level
 * declarations is split into one file per declaration (`private` becomes `internal` so the split still compiles).
 * The split module is checked in full by a plain session (the reference) and by a recording session, whose check also
 * builds the binary snapshot. Then each piece is checked alone with that session's `sliceCheck`, against the
 * snapshot of the others, and its verdict is compared with the reference. The snapshot and the slice compile are
 * the production ones. A difference means a rule used a fact of another declaration that only its source
 * carries (an initializer, a body, its source element), so the rule must be tiered: every rule that differs must
 * have [dev.jasonpearson.krit.fir.FirRule.wholeCompilationTier] set. The compiler-error gate must never differ.
 *
 * The single-file split never separates a property from the code that uses it, so [crossFileScenarios] (two files:
 * the declaration in B, the use in A) run too. Known-different scenarios must differ only in tiered rules; the
 * safe controls (const values, literal-null initializers, annotation defaults) must not differ at all.
 *
 * With the full corpus (no sampling or filter) the run also fails when it is vacuous: too few rules produced
 * findings, or no piece differed, which means the harness stopped loading the plugin or the snapshot.
 *
 * Runtime: a few minutes, so it is tagged [TAG] and runs in its own Gradle task (`incrementalDifferentialTest`),
 * not in `test`. `-Pkrit.fir.differential.sample=N` keeps every Nth scenario (deterministic, sorted by path);
 * `-Pkrit.fir.differential.only=Name` keeps scenarios whose file name contains `Name`.
 */
@Tag(FirIncrementalSplitDifferentialTest.TAG)
class FirIncrementalSplitDifferentialTest {
    companion object {
        /** The spike saw 77 rules with findings; far fewer means the plugin or the corpus did not load. */
        private const val MIN_EXERCISED_RULES = 60
        const val TAG = "incremental-differential"
        private val DATA = File("compiler-tests/src/test/data")
        private val HEADER_MARKERS = Regex("<![A-Za-z_][A-Za-z0-9_, ]*!>")
    }

    @Test
    fun onlyTieredRulesDifferBetweenFullAndSliceCompile() {
        assertTrue(DATA.isDirectory, "run from tools/krit-fir; missing ${DATA.absolutePath}")
        val sample = System.getProperty("krit.fir.differential.sample")?.toIntOrNull()?.coerceAtLeast(1) ?: 1
        val only = System.getProperty("krit.fir.differential.only")?.takeIf { it.isNotBlank() }
        val tiered = FirRuleDiscovery.wholeCompilationRuleIds().toSet()

        val work = Files.createTempDirectory("krit-fir-differential").toFile()
        val disposable = Disposer.newDisposable("differential-parse")
        try {
            val cp = listOf(compileStubs(work).path)
            val psi = KtPsiFactory(parseEnvironment(disposable).project)
            val cases = File(DATA, "diagnostic").walkTopDown().filter { it.isFile && it.extension == "kt" }
                .sortedBy { it.path }
                .filter { only == null || it.nameWithoutExtension.contains(only) }
                .filterIndexed { i, _ -> i % sample == 0 }
                .toList()
            val result = Result()
            for (case in cases) compareCase(case, psi, cp, work, result)

            // Two-file scenarios: the declaration in B, the use in A.
            val scenarioFailures = ArrayList<String>()
            val scenarios = crossFileScenarios.filter { only == null || it.name.contains(only) || it.rule.contains(only) }
            for (s in scenarios) {
                val diffs = diffModule(s.files, cp, File(work, "scenario"), s.config, result)
                val byRule = TreeMap<String, MutableList<String>>()
                for (d in diffs) for ((rule, what) in d.byRule) byRule.getOrPut(rule) { ArrayList() }.add("${d.file}: $what")
                println("## scenario '${s.name}': ${if (byRule.isEmpty()) "same" else "differs in " + byRule.keys}")
                for ((rule, xs) in byRule) {
                    if (rule !in tiered) scenarioFailures += "${s.name}: $rule differs but is not tiered: ${xs.first()}"
                }
                val ownRule = byRule[s.rule].orEmpty()
                if (s.differs && ownRule.isEmpty()) {
                    scenarioFailures += "${s.name}: expected ${s.rule} to differ on a slice, but it did not. If the rule " +
                        "was fixed, turn this scenario into a control (differs = false) and drop its tier if no read remains."
                }
                if (!s.differs && ownRule.isNotEmpty()) {
                    scenarioFailures += "${s.name}: control for ${s.rule} differs between full and slice: ${ownRule.first()}"
                }
                for (d in diffs) if (d.errorDiff != null) scenarioFailures += "${s.name}: compiler-error gate differs: ${d.errorDiff}"
            }

            println(
                "## split differential: scenarios=${cases.size} split=${result.split} compared=${result.compared} " +
                    "pieces=${result.pieces} pieceVerdictDiffs=${result.pieceDiffs} " +
                    "rulesWithFindings=${result.exercised.size} crossFileScenarios=${scenarios.size}",
            )
            for ((rule, xs) in result.diffByRule) {
                println("## DIFF $rule (${if (rule in tiered) "tiered" else "NOT TIERED"}): ${xs.size} findings differ")
                xs.take(3).forEach { println("     $it") }
            }
            assertTrue(result.compared > 0, "no scenario could be split and compared; the harness is broken")
            if (sample == 1 && only == null) {
                assertTrue(
                    result.exercised.size >= MIN_EXERCISED_RULES,
                    "Only ${result.exercised.size} rules produced findings (expected at least $MIN_EXERCISED_RULES): " +
                        "the harness probably stopped loading the krit plugin.",
                )
                assertTrue(
                    result.pieceDiffs > 0,
                    "No piece differed between the full and the slice compile. The tiered rules are known to differ, " +
                        "so the slice compile is probably not reading the snapshot.",
                )
            }
            val untiered = result.diffByRule.filterKeys { it !in tiered }
            assertTrue(
                untiered.isEmpty(),
                "These rules give a different verdict on a slice compile against a binary snapshot but are not " +
                    "tiered. Set `override val wholeCompilationTier = otherDeclarationBodies(\"<why>\")` on each rule " +
                    "(the reason is the other-file fact it reads), or make the rule read only the file under check:\n" +
                    untiered.entries.joinToString("\n") { (rule, xs) -> "$rule (${xs.size}):\n    " + xs.take(4).joinToString("\n    ") },
            )
            assertTrue(
                result.errorDiffs.isEmpty(),
                "The compiler-error gate (which requested files have a compile error) differs between a full and a " +
                    "slice compile, so an incremental run could not trust it:\n" + result.errorDiffs.take(10).joinToString("\n"),
            )
            assertTrue(scenarioFailures.isEmpty(), "Cross-file scenarios:\n" + scenarioFailures.joinToString("\n"))
            assertTrue(
                result.coldDiffs.isEmpty(),
                "The check that also builds the snapshot gave a different verdict than a plain check:\n" +
                    result.coldDiffs.take(10).joinToString("\n"),
            )
        } finally {
            Disposer.dispose(disposable)
            work.deleteRecursively()
        }
    }

    private class Result {
        var split = 0
        var compared = 0
        var pieces = 0
        var pieceDiffs = 0
        val diffByRule = TreeMap<String, MutableList<String>>()
        val exercised = TreeMap<String, Int>()
        val errorDiffs = ArrayList<String>()
        val coldDiffs = ArrayList<String>()
    }

    /** One file whose slice verdict differs from the full-compile verdict. */
    private class PieceDiff(val file: String, val byRule: Map<String, String>, val errorDiff: String?)

    /** The shared Kotlin stubs, compiled once into a class directory (Java stubs are source roots). */
    private fun compileStubs(work: File): File {
        val out = File(work, "kt-stubs").also { it.mkdirs() }
        val stubSources = File(DATA, "stubs").listFiles { f -> f.extension == "kt" }!!.map { it.path }.sorted()
        compileKotlinLibrary(stubSources, out, listOf(File(DATA, "stubs/java").path))
        assertTrue(out.walkTopDown().any { it.extension == "class" }, "stub library did not compile")
        return out
    }

    private fun parseEnvironment(disposable: com.intellij.openapi.Disposable): KotlinCoreEnvironment {
        val cfg = CompilerConfiguration().also { it.extensionsStorage = CompilerPluginRegistrar.ExtensionStorage() }
        return KotlinCoreEnvironment.createForProduction(disposable, cfg, EnvironmentConfigFiles.JVM_CONFIG_FILES)
    }

    private fun compareCase(case: File, psi: KtPsiFactory, cp: List<String>, work: File, r: Result) {
        val text = case.readText().replace(HEADER_MARKERS, "").replace("<!>", "")
        val kt = psi.createFile(case.name, text)
        val decls = kt.declarations
        if (decls.size < 2) return
        r.split++
        val header = buildString {
            kt.fileAnnotationList?.annotationEntries?.filter { !it.text.contains("JvmName") }
                ?.forEach { append(it.text).append('\n') }
            kt.packageDirective?.let { append(it.text).append('\n') }
            kt.importList?.let { append(it.text).append('\n') }
            append('\n')
        }
        val pieces = LinkedHashMap<String, String>()
        decls.forEachIndexed { i, d ->
            var body = d.text
            val priv = (d as? KtModifierListOwner)?.modifierList?.getModifier(KtTokens.PRIVATE_KEYWORD)
            if (priv != null) {
                val off = priv.textRange.startOffset - d.textRange.startOffset
                body = body.substring(0, off) + "internal" + body.substring(off + "private".length)
            }
            pieces["P${i}_${(d.name ?: "anon").take(30)}.kt"] = header + body + "\n"
        }
        val tag = case.relativeTo(File(DATA, "diagnostic")).path
        val diffs = diffModule(pieces, cp, File(work, "case"), emptyMap(), r, tagPrefix = tag)
        for (d in diffs) {
            for ((rule, what) in d.byRule) r.diffByRule.getOrPut(rule) { ArrayList() }.add("${d.file} $what")
            if (d.errorDiff != null) r.errorDiffs += d.errorDiff
        }
    }

    /**
     * Checks [sources] (file name to text) in full as the reference, and again in a recording session whose check
     * builds the snapshot, and then checks each file alone against the snapshot of the others. Returns the files
     * whose verdict differs. Counts go into [r].
     */
    private fun diffModule(
        sources: Map<String, String>,
        cp: List<String>,
        rootDir: File,
        ruleConfigs: Map<String, Map<String, Any?>>,
        r: Result,
        tagPrefix: String = "",
    ): List<PieceDiff> {
        val root = rootDir.also { it.deleteRecursively(); it.mkdirs() }
        val srcDir = File(root, "src")
        val files = sources.map { (name, text) ->
            File(srcDir, name).also { it.parentFile.mkdirs(); it.writeText(text) }.path
        }.map(::canon)
        val javaRoots = arrayOf(File(DATA, "stubs/java").path)
        val requested = files.map { FileRef(it) }
        fun session(record: Boolean) = AnalysisSession(listOf(srcDir.path), cp, "", emptyList(), incrementalRecording = record).also {
            it.snapshotRootOverride = File(root, "snapshots")
            it.argumentsHook = { args -> args.javaSourceRoots = javaRoots }
        }

        // Reference: the whole module from source, frontend only, nothing recorded.
        val ref = session(record = false).check(1, requested, emptySet(), ruleConfigs)
        for (f in ref.findings) r.exercised.merge(f.rule, 1, Int::plus)

        // Snapshot: the same check in a recording session, which also writes the binaries.
        val session = session(record = true)
        try {
            val full = session.check(1, requested, emptySet(), ruleConfigs)
            // The check that builds the snapshot must give the verdicts of a plain check, snapshot or not.
            for (p in files) {
                if (ref.verdictOf(p).key() != full.verdictOf(p).key()) r.coldDiffs += "${File(p).name}: ${ref.verdictOf(p)} vs ${full.verdictOf(p)}"
            }
            if (session.incrementalState.snapshot == null) {
                // Compile errors (or expect/actual) have no snapshot to compare against; a clean module must have one.
                if (ref.errorFiles.isEmpty() && session.incrementalState.fallbackReason() != SliceFallback.EXPECT_ACTUAL) {
                    r.coldDiffs += "${sources.keys.first()}: a clean module got no snapshot (${session.incrementalState.fallbackReason()})"
                }
                return emptyList()
            }
            r.compared++

            val diffs = ArrayList<PieceDiff>()
            for (p in files) {
                r.pieces++
                val outcome = session.sliceCheck(1, listOf(FileRef(p)), emptySet(), ruleConfigs)
                val inc = (outcome as? SliceOutcome.Checked)?.result
                    ?: error("slice of ${File(p).name} fell back: ${(outcome as SliceOutcome.Fallback).reason} ${outcome.cause}")
                val expected = ref.verdictOf(p)
                val actual = inc.verdictOf(p)
                if (expected.key() == actual.key()) continue
                r.pieceDiffs++
                val tag = (if (tagPrefix.isEmpty()) "" else "$tagPrefix:") + File(p).name
                val byRule = LinkedHashMap<String, String>()
                val onlyFull = expected.findings - actual.findings.toSet()
                val onlySlice = actual.findings - expected.findings.toSet()
                for (f in onlyFull) byRule.putIfAbsent(f.substringBefore('@'), "full-only ${f.take(140)}")
                for (f in onlySlice) byRule.putIfAbsent(f.substringBefore('@'), "slice-only ${f.take(140)}")
                // A rule that crashed on one side only.
                for (rule in (expected.ruleErrors - actual.ruleErrors) + (actual.ruleErrors - expected.ruleErrors)) {
                    byRule.putIfAbsent(rule, "rule error differs")
                }
                var errorDiff: String? = null
                if (expected.error != actual.error) {
                    errorDiff = "$tag error full=${expected.error} slice=${actual.error} ${actual.errorMessage?.take(160)}"
                }
                diffs += PieceDiff(tag, byRule, errorDiff)
            }
            return diffs
        } finally {
            session.dispose()
        }
    }
}
