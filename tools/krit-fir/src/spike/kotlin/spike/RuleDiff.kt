@file:Suppress("DEPRECATION")
@file:OptIn(org.jetbrains.kotlin.CoreEnvironmentDeprecation::class, org.jetbrains.kotlin.config.CompilerConfiguration.Internals::class, org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class)

package spike

import com.intellij.openapi.util.Disposer
import org.jetbrains.kotlin.cli.jvm.compiler.EnvironmentConfigFiles
import org.jetbrains.kotlin.cli.jvm.compiler.KotlinCoreEnvironment
import org.jetbrains.kotlin.config.CompilerConfiguration
import org.jetbrains.kotlin.cli.extensionsStorage
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.psi.KtModifierListOwner
import org.jetbrains.kotlin.psi.KtPsiFactory
import java.io.File

/**
 * Which krit rules give a different verdict for a file when the rest of its module comes from class files (F2's slice
 * against a binary snapshot, and F3's IC) instead of source? Corpus: krit-fir's own compiler-test data (one file per
 * rule scenario). Each test-data file is split into one file per top-level declaration (private -> internal so the
 * split still compiles), the split module is compiled in full (frontend only, the reference) and with the backend (the
 * snapshot), then every piece is compiled alone against the snapshot through the stock IC path and its verdict is
 * compared with the reference. A difference means the rule (or the compiler) used a fact of another declaration that
 * only its source carries: an initializer, a body, its PSI / source text.
 */
class RuleDiff(val o: Map<String, String>) {
    val data = File(o["data"] ?: "/home/user/krit/.claude/worktrees/agent-acce44a119e438fc1/tools/krit-fir/compiler-tests/src/test/data")
    val dir = File(SCRATCH, "rulediff")
    val only = o["only"]

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        val javaStubs = File(data, "stubs/java")
        val ktStubsOut = File(dir, "kt-stubs").also { it.mkdirs() }
        val stubSources = File(data, "stubs").listFiles { f -> f.extension == "kt" }!!.map { it.path }
        val sa = kritArgs(stubSources, emptyList(), ktStubsOut).also { it.javaSourceRoots = arrayOf(javaStubs.path); it.pluginClasspaths = emptyArray() }
        val stubRes = runShipped(sa, emptyList(), frontendOnly = false)
        println("# kotlin stubs: ${stubSources.size} files -> exit=${stubRes.exit} classes=${ktStubsOut.walkTopDown().count { it.extension == "class" }}")
        val cp = listOf(ktStubsOut.path)

        val disposable = Disposer.newDisposable("rulediff-parse")
        val cfg = CompilerConfiguration().also { it.extensionsStorage = org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar.ExtensionStorage() }
        val env = KotlinCoreEnvironment.createForProduction(disposable, cfg, EnvironmentConfigFiles.JVM_CONFIG_FILES)
        val psi = KtPsiFactory(env.project)

        val cases = File(data, "diagnostic").walkTopDown().filter { it.isFile && it.extension == "kt" }.sortedBy { it.path }
            .filter { only == null || it.nameWithoutExtension.contains(only) }.toList()
        var split = 0; var compared = 0; var noSnapshot = 0; var pieceCount = 0; var pieceDiffs = 0
        val diffByRule = java.util.TreeMap<String, MutableList<String>>()
        val exercised = java.util.TreeMap<String, Int>()
        val errorDiffs = ArrayList<String>()
        for ((ci, case) in cases.withIndex()) {
            val text = case.readText().replace(Regex("<![A-Za-z_][A-Za-z0-9_, ]*!>"), "").replace("<!>", "")
            val kt = psi.createFile(case.name, text)
            val decls = kt.declarations
            if (decls.size < 2) continue
            split++
            val header = buildString {
                kt.fileAnnotationList?.annotationEntries?.filter { !it.text.contains("JvmName") }?.forEach { append(it.text).append('\n') }
                kt.packageDirective?.let { append(it.text).append('\n') }
                kt.importList?.let { append(it.text).append('\n') }
                append('\n')
            }
            val root = File(dir, "case").also { it.deleteRecursively(); it.mkdirs() }
            val pieces = decls.mapIndexed { i, d ->
                var body = d.text
                val priv = (d as? KtModifierListOwner)?.modifierList?.getModifier(KtTokens.PRIVATE_KEYWORD)
                if (priv != null) {
                    val off = priv.textRange.startOffset - d.textRange.startOffset
                    body = body.substring(0, off) + "internal" + body.substring(off + "private".length)
                }
                File(root, "src/P${i}_${(d.name ?: "anon").take(30)}.kt").also { it.parentFile.mkdirs(); it.writeText(header + body + "\n") }.path
            }.map(::canon)
            fun args(files: List<String>, out: File) = kritArgs(files, cp, out).also { it.javaSourceRoots = arrayOf(javaStubs.path) }
            val ref = runShipped(args(pieces, File(root, "ref")), pieces, frontendOnly = true)
            for (v in ref.verdicts.values) for (f in v.findings) exercised.merge(f.substringBefore('@'), 1, Int::plus)
            val tmp = File(root, "tmp").also { it.mkdirs() }
            val full = runCheck(StockPhases.full(null), args(pieces, tmp).also { it.reportOutputFiles = true }, pieces)
            if (full.outputs.isEmpty()) { noSnapshot++; continue }
            val snapDir = File(root, "snapshot"); tmp.renameTo(snapDir)
            val snap = Snapshot(snapDir, "main")
            snap.adopt(full.outputs.mapValues { (_, ps) -> ps.map { it.replace(tmp.path, snapDir.path) } })
            compared++
            for (p in pieces) {
                pieceCount++
                snap.park(listOf(p))
                val a = args(listOf(p), File(root, "slice")).also { it.incrementalCompilation = true; it.classpath = it.classpath + File.pathSeparator + snapDir.path }
                val inc = runCheck(StockPhases.frontend(null), a, listOf(p), services(null, snap))
                snap.unpark()
                val r = ref.verdicts[p]; val i = inc.verdicts[p]
                if (r?.verdictKey() == i?.verdictKey()) continue
                pieceDiffs++
                val onlyRef = r?.findings.orEmpty() - i?.findings.orEmpty().toSet()
                val onlyInc = i?.findings.orEmpty() - r?.findings.orEmpty().toSet()
                val tag = "${case.relativeTo(File(data, "diagnostic")).path}:${File(p).name}"
                for (f in onlyRef) diffByRule.getOrPut(f.substringBefore('@')) { ArrayList() }.add("$tag full-only ${f.take(140)}")
                for (f in onlyInc) diffByRule.getOrPut(f.substringBefore('@')) { ArrayList() }.add("$tag slice-only ${f.take(140)}")
                if (r?.error != i?.error) errorDiffs += "$tag error full=${r?.error} slice=${i?.error} ${(i?.diags.orEmpty() - r?.diags.orEmpty().toSet()).firstOrNull()?.take(160)}"
            }
            if (ci % 50 == 0) println("progress ${ci + 1}/${cases.size} split=$split compared=$compared pieces=$pieceCount diffs=$pieceDiffs load=${load1()}")
        }
        println("## rulediff: testData=${cases.size} splittable=$split compared=$compared (no snapshot because of errors: $noSnapshot) pieces=$pieceCount pieceVerdictDiffs=$pieceDiffs")
        println("## rules with findings in the split corpus: ${exercised.size}: $exercised")
        for ((rule, xs) in diffByRule) {
            println("## DIFF $rule: ${xs.size} findings differ")
            xs.take(6).forEach { println("     $it") }
        }
        println("## compiler-error gate differences: ${errorDiffs.size}")
        errorDiffs.take(15).forEach { println("     $it") }
        Disposer.dispose(disposable)
        dir.deleteRecursively()
    }
}
