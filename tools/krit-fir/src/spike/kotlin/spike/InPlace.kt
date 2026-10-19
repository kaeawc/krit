@file:OptIn(org.jetbrains.kotlin.fir.SessionConfiguration::class)

package spike

import org.jetbrains.kotlin.KtIoFileSourceFile
import org.jetbrains.kotlin.cli.common.ExitCode
import org.jetbrains.kotlin.cli.common.fir.FirDiagnosticsCompilerResultsReporter
import org.jetbrains.kotlin.diagnostics.impl.DiagnosticsCollectorImpl
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.pipeline.SingleModuleFrontendOutput
import org.jetbrains.kotlin.fir.pipeline.buildFirViaLightTree
import org.jetbrains.kotlin.fir.pipeline.runCheckers
import org.jetbrains.kotlin.fir.pipeline.runResolution
import org.jetbrains.kotlin.fir.resolve.providers.FirSymbolProvider
import org.jetbrains.kotlin.fir.resolve.providers.firProvider
import org.jetbrains.kotlin.fir.resolve.providers.impl.FirCachingCompositeSymbolProvider
import org.jetbrains.kotlin.fir.resolve.providers.impl.FirProviderImpl
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import java.io.File

/**
 * F4 (time-boxed): update a CLI FIR source session in place. After one full warm compile the session is kept; per edit
 * the edited files and their dependents are re-parsed into new raw FirFiles in the *same* session, the provider index
 * is rebuilt from the new file list (`FirProviderImpl.ensureConsistent`, @TestOnly, which re-records every file and
 * swaps the index), the session's caching composite symbol provider is replaced by a fresh one (its caches are
 * otherwise permanent), and only the new files are resolved and checked.
 *
 * Dependents: the compiler LookupTracker reverse index (recorded by the cold compile and by every in-place resolve),
 * ungated (every lookup of a name the edited file declares, declared before or after), plus files declaring the same
 * top-level names. The per-file verdict cache is compared with a fresh full frontend compile after every edit (all files).
 */
class InPlace(val o: Map<String, String>) {
    val n = (o["files"] ?: "300").toInt()
    val edits = (o["edits"] ?: "11").toInt()
    val dir = File(SCRATCH, "inplace-$n")
    val swapProvider = o["swapProvider"] != "false"
    val kotlinArgs = o["kotlinArgs"]?.split(",")?.filter { it.isNotBlank() } ?: emptyList()

    private val declRe = Regex("""(?m)^\s*(?:(?:public|internal|private|protected|open|abstract|sealed|data|enum|inline|value|annotation|fun|const|override|suspend|operator|infix|lateinit|expect|actual|companion)\s+)*(?:class|object|interface|fun|val|var|typealias)\s+(?:<[^>]*>\s*)?(?:[A-Za-z_][A-Za-z0-9_.]*\.)?([A-Za-z_][A-Za-z0-9_]*)""")

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        val g = makeProj(o, File(dir, "src"), n, (o["seed"] ?: "7").toLong())
        println("# ${g.describe()} swapProvider=$swapProvider")
        val all0 = g.texts.keys.map(::canon)
        val st = WarmState()
        var out: SingleModuleFrontendOutput? = null
        val outDir = File(dir, "out").also { it.mkdirs() }
        val lookups = CompileLookups()
        val cold = runCheck(Phases.warmFrontend(st, null, Timings()) { out = it }, kritArgs(all0, g.classpath, outDir, kotlinArgs = kotlinArgs), all0, services(lookups, null))
        st.releaseHeldCompiles()
        val session = out!!.session
        val index = LookupIndex(); for (f in all0) index.update(f, lookups.byFile[f] ?: emptySet())
        val cache = HashMap(cold.verdicts)
        println("cold: exit=${cold.exit} findings=${cold.verdicts.values.sumOf { it.findings.size }} errFiles=${cold.verdicts.values.count { it.error }} files=${out!!.fir.size} lookups=${lookups.records}")
        val firByPath = LinkedHashMap<String, FirFile>()
        for (f in out!!.fir) firByPath[canon(f.sourceFile!!.path!!)] = f
        out = null
        val kinds = o["kinds"]?.split(",")
        var identical = 0; var differs = 0; var crashed = 0; var staleFiles = 0
        val times = ArrayList<Long>()
        for (i in 1..edits) {
            val names = HashSet<String>()
            val before = HashMap(g.texts)
            val e = if (kinds != null && o["fixedKinds"] == "true") (g as Gen).apply(kinds[(i - 1) % kinds.size]) else g.nextEdit(kinds)
            val E = (e.changed + e.added).filter { File(it).exists() }.map(::canon)
            val D = e.deleted.map(::canon)
            // names declared by the edited files before and after the edit
            for (p in e.changed + e.added + e.deleted) {
                (before[p] ?: "").let { t -> declRe.findAll(t).forEach { names += it.groupValues[1] } }
                (g.texts[p] ?: "").let { t -> declRe.findAll(t).forEach { names += it.groupValues[1] } }
            }
            val byName = index.dependentsByName(names)
            val sameName = g.texts.entries.filter { (p, t) -> canon(p) !in E && declRe.findAll(t).any { it.groupValues[1] in names } }.map { canon(it.key) }
            val rebuild = (E + byName + sameName).distinct().filter { it !in D && File(it).exists() }
            val t0 = System.nanoTime()
            val result = try {
                val reporter = DiagnosticsCollectorImpl()
                val newFiles = session.buildFirViaLightTree(rebuild.map { KtIoFileSourceFile(File(it)) }, reporter, null)
                for (d in D) firByPath.remove(d)
                for (nf in newFiles) firByPath[canon(nf.sourceFile!!.path!!)] = nf
                (session.firProvider as FirProviderImpl).ensureConsistent(firByPath.values.toList())
                if (swapProvider) {
                    val old = session.symbolProvider as FirCachingCompositeSymbolProvider
                    session.register(FirSymbolProvider::class, FirCachingCompositeSymbolProvider(session, old.providers))
                }
                for (f in rebuild) lookups.byFile.remove(f)
                val k = KritCheck(rebuild)
                val exit = k.run { mc ->
                    val (scopeSession, fir) = session.runResolution(newFiles)
                    session.runCheckers(scopeSession, fir, reporter, MppCheckerKind.Common)
                    FirDiagnosticsCompilerResultsReporter.reportToMessageCollector(reporter, mc, false)
                    ExitCode.OK
                }
                k.out(exit)
            } catch (t: Throwable) {
                crashed++
                println("e$i ${e.kind}: in-place update threw ${t.javaClass.simpleName}: ${t.message?.lineSequence()?.firstOrNull()?.take(200)}")
                t.stackTrace.take(6).forEach { println("      at $it") }
                null
            }
            val d = System.nanoTime() - t0
            times += d
            // the kept session's lookup tracker component still reports into `lookups`: refresh the re-resolved files
            for (f in D) { cache.remove(f); index.remove(f) }
            if (result != null) for (f in rebuild) index.update(f, lookups.byFile[f] ?: emptySet())
            if (result != null) for (f in rebuild) result.verdicts[f]?.let { cache[f] = it }
            val all = g.texts.keys.map(::canon)
            val ref = runShipped(kritArgs(all, g.classpath, outDir, kotlinArgs = kotlinArgs), all, frontendOnly = true)
            val diffRebuilt = if (result != null) Compare.strictDiff(ref.verdicts, result.verdicts, rebuild) else rebuild
            val diffAll = Compare.verdictDiff(ref.verdicts, cache, all)
            if (diffRebuilt.isEmpty() && diffAll.isEmpty()) identical++ else differs++
            staleFiles += diffAll.size
            println("e$i ${e.kind}: rebuilt=${rebuild.size} ${ms(d)} ms rebuilt-files ${if (diffRebuilt.isEmpty()) "IDENTICAL" else "DIFFER in ${diffRebuilt.size}"}; whole-module verdicts stale=${diffAll.size} load=${load1()}")
            diffRebuilt.take(2).forEach { println("     rebuilt: " + Compare.explain(it, ref.verdicts[it], result?.verdicts?.get(it))) }
            diffAll.take(2).forEach { println("     cached:  " + Compare.explain(it, ref.verdicts[it], cache[it])) }
            if (o["resync"] == "true") cache.putAll(ref.verdicts)
            if (i % 20 == 0) println("mem@e$i: ${memSnap(true)}")
        }
        println("## in-place: identical=$identical differs=$differs crashed=$crashed of $edits edits; stale cached verdicts=$staleFiles; update p50=${ms(pct(times, .5))} p95=${ms(pct(times, .95))} ms (swapProvider=$swapProvider)")
        println("## memory: ${memSnap(true)}")
        st.dispose()
        dir.deleteRecursively()
    }
}
