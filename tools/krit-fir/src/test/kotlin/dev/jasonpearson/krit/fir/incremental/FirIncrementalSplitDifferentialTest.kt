@file:OptIn(
    org.jetbrains.kotlin.CoreEnvironmentDeprecation::class,
    org.jetbrains.kotlin.config.CompilerConfiguration.Internals::class,
    org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class,
)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import com.intellij.openapi.util.Disposer
import dev.jasonpearson.krit.fir.FirRuleDiscovery
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
 * The split module is compiled in full (frontend only: the reference) and with the backend (the snapshot). Then each
 * piece is compiled alone through the stock incremental path against the snapshot of the others, and its verdict is
 * compared with the reference. A difference means a rule used a fact of another declaration that only its source
 * carries (an initializer, a body, its source element), so the rule must be tiered: every rule that differs must
 * have [dev.jasonpearson.krit.fir.FirRule.wholeCompilationReason] set. The compiler-error gate must never differ.
 *
 * Runtime: a few minutes, so it is tagged [TAG] and runs in its own Gradle task (`incrementalDifferentialTest`),
 * not in `test`. `-Pkrit.fir.differential.sample=N` keeps every Nth scenario (deterministic, sorted by path);
 * `-Pkrit.fir.differential.only=Name` keeps scenarios whose file name contains `Name`.
 */
@Tag(FirIncrementalSplitDifferentialTest.TAG)
class FirIncrementalSplitDifferentialTest {
    companion object {
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

            println(
                "## split differential: scenarios=${cases.size} split=${result.split} compared=${result.compared} " +
                    "pieces=${result.pieces} pieceVerdictDiffs=${result.pieceDiffs} " +
                    "rulesWithFindings=${result.exercised.size}",
            )
            for ((rule, xs) in result.diffByRule) {
                println("## DIFF $rule (${if (rule in tiered) "tiered" else "NOT TIERED"}): ${xs.size} findings differ")
                xs.take(3).forEach { println("     $it") }
            }
            assertTrue(result.compared > 0, "no scenario could be split and compared; the harness is broken")
            val untiered = result.diffByRule.filterKeys { it !in tiered }
            assertTrue(
                untiered.isEmpty(),
                "These rules give a different verdict on a slice compile against a binary snapshot but are not " +
                    "tiered. Set `override val wholeCompilationReason = \"<why>\"` on each rule (the reason is the " +
                    "other-file fact it reads), or make the rule read only the file under check:\n" +
                    untiered.entries.joinToString("\n") { (rule, xs) -> "$rule (${xs.size}):\n    " + xs.take(4).joinToString("\n    ") },
            )
            assertTrue(
                result.errorDiffs.isEmpty(),
                "The compiler-error gate (which requested files have a compile error) differs between a full and a " +
                    "slice compile, so an incremental run could not trust it:\n" + result.errorDiffs.take(10).joinToString("\n"),
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
    }

    /** The shared Kotlin stubs, compiled once into a class directory (Java stubs are source roots). */
    private fun compileStubs(work: File): File {
        val out = File(work, "kt-stubs").also { it.mkdirs() }
        val stubSources = File(DATA, "stubs").listFiles { f -> f.extension == "kt" }!!.map { it.path }.sorted()
        val args = kritArgs(stubSources, emptyList(), out).also {
            it.javaSourceRoots = arrayOf(File(DATA, "stubs/java").path)
            it.pluginClasspaths = emptyArray()
        }
        runShipped(args, emptyList(), frontendOnly = false)
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
        val root = File(work, "case").also { it.deleteRecursively(); it.mkdirs() }
        val pieces = decls.mapIndexed { i, d ->
            var body = d.text
            val priv = (d as? KtModifierListOwner)?.modifierList?.getModifier(KtTokens.PRIVATE_KEYWORD)
            if (priv != null) {
                val off = priv.textRange.startOffset - d.textRange.startOffset
                body = body.substring(0, off) + "internal" + body.substring(off + "private".length)
            }
            File(root, "src/P${i}_${(d.name ?: "anon").take(30)}.kt")
                .also { it.parentFile.mkdirs(); it.writeText(header + body + "\n") }.path
        }.map(::canon)
        val javaRoots = arrayOf(File(DATA, "stubs/java").path)
        fun args(files: List<String>, out: File) = kritArgs(files, cp, out).also { it.javaSourceRoots = javaRoots }

        // Reference: the whole split module from source, frontend only.
        val ref = runShipped(args(pieces, File(root, "ref")), pieces, frontendOnly = true)
        for (v in ref.verdicts.values) for (f in v.findings) r.exercised.merge(f.substringBefore('@'), 1, Int::plus)

        // Snapshot: the same module with the backend, class files plus the source -> output map.
        val tmp = File(root, "tmp").also { it.mkdirs() }
        val full = runCheck(StockPhases.full, args(pieces, tmp).also { it.reportOutputFiles = true }, pieces)
        if (full.outputs.isEmpty()) return // the split module has compile errors: no snapshot to compare against
        val snapDir = File(root, "snapshot")
        tmp.renameTo(snapDir)
        val snap = Snapshot(snapDir)
        snap.adopt(full.outputs.mapValues { (_, ps) -> ps.map { it.replace(tmp.path, snapDir.path) } })
        r.compared++

        for (p in pieces) {
            r.pieces++
            snap.park(listOf(p))
            val a = args(listOf(p), File(root, "slice")).also {
                it.incrementalCompilation = true
                it.classpath = it.classpath + File.pathSeparator + snapDir.path
            }
            val inc = try {
                runCheck(StockPhases.frontend, a, listOf(p), snapshotServices(snap))
            } finally {
                snap.unpark()
            }
            val expected = ref.verdicts[p]
            val actual = inc.verdicts[p]
            if (expected?.verdictKey() == actual?.verdictKey()) continue
            r.pieceDiffs++
            val tag = "${case.relativeTo(File(DATA, "diagnostic")).path}:${File(p).name}"
            val onlyFull = expected?.findings.orEmpty() - actual?.findings.orEmpty().toSet()
            val onlySlice = actual?.findings.orEmpty() - expected?.findings.orEmpty().toSet()
            for (f in onlyFull) r.diffByRule.getOrPut(f.substringBefore('@')) { ArrayList() }.add("$tag full-only ${f.take(140)}")
            for (f in onlySlice) r.diffByRule.getOrPut(f.substringBefore('@')) { ArrayList() }.add("$tag slice-only ${f.take(140)}")
            // A rule that crashed on one side only.
            val ruleErrorDiff = expected?.ruleErrors?.keys.orEmpty().toSet() symmetricDifference actual?.ruleErrors?.keys.orEmpty().toSet()
            for (rule in ruleErrorDiff) r.diffByRule.getOrPut(rule) { ArrayList() }.add("$tag rule error differs")
            if (expected?.error != actual?.error) {
                val extra = (actual?.diags.orEmpty() - expected?.diags.orEmpty().toSet()).firstOrNull()?.take(160)
                r.errorDiffs += "$tag error full=${expected?.error} slice=${actual?.error} $extra"
            }
        }
    }

    private infix fun <T> Set<T>.symmetricDifference(other: Set<T>): Set<T> = (this - other) + (other - this)
}
