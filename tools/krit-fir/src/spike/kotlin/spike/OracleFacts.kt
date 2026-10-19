package spike

import dev.jasonpearson.krit.fir.oracle.OracleCollector
import dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry
import java.io.File

/**
 * Q7: does a slice compile against the binary snapshot still produce krit-fir's oracle facts (the `analyzeWithCheck`
 * payload: per-file declarations, expression types, diagnostics, lambda flags, and DepTracker's depPathsByFile /
 * perFileDeps) for the files it compiles? Compares each sampled file's payload from a slice compile with the one from
 * a full frontend compile of the module.
 */
class OracleFacts(val o: Map<String, String>) {
    val n = (o["files"] ?: "300").toInt()
    val sample = (o["sample"] ?: "20").toInt()
    val kotlinArgs = o["kotlinArgs"]?.split(",")?.filter { it.isNotBlank() } ?: emptyList()
    val dir = File(SCRATCH, "oraclefacts")

    private fun collect(body: () -> CheckOut): Pair<OracleCollector, CheckOut> {
        val c = OracleCollector()
        OracleCollectorRegistry.begin(c)
        try { return c to body() } finally { OracleCollectorRegistry.end() }
    }

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        val g = makeProj(o, File(dir, "src"), n, 7L)
        println("# ${g.describe()}")
        val all = g.texts.keys.map(::canon)
        val (full, fullOut) = collect { runShipped(kritArgs(all, g.classpath, File(dir, "ref"), kotlinArgs = kotlinArgs), all, frontendOnly = true) }
        val fullRes = full.toResult()
        val fullFiles = fullRes.files.mapKeys { canon(it.key) }
        val fullDeps = full.depTracker.depPathsByFile.entries.associate { canon(it.key) to it.value.map(::canon).toSet() }
        println("full: exit=${fullOut.exit} payloadFiles=${fullFiles.size} filesWithDepPaths=${fullDeps.size} depEdges=${fullDeps.values.sumOf { it.size }}")
        val tmp = File(dir, "tmp").also { it.mkdirs() }
        val snapCompile = runCheck(StockPhases.full(null), kritArgs(all, g.classpath, tmp, kotlinArgs = kotlinArgs).also { it.reportOutputFiles = true }, all)
        if (snapCompile.outputs.isEmpty()) { println("no snapshot: the module does not compile without errors (${snapCompile.firstError?.take(160)})"); return }
        val snapDir = File(dir, "snapshot"); tmp.renameTo(snapDir)
        val snap = Snapshot(snapDir, "main"); snap.adopt(snapCompile.outputs.mapValues { (_, ps) -> ps.map { it.replace(tmp.path, snapDir.path) } })
        // prefer files with DepTracker edges, then the rest
        val only = o["only"]?.split(",")
        val picks = if (only != null) all.filter { File(it).name in only } else (fullDeps.keys.shuffled(java.util.Random(3)).take(sample / 2) + all.shuffled(java.util.Random(5))).distinct().take(sample)
        var same = 0; var declDiff = 0; var exprDiff = 0; var diagDiff = 0; var lambdaDiff = 0; var depDiff = 0
        for (f in picks) {
            snap.park(listOf(f))
            val args = kritArgs(listOf(f), g.classpath, File(dir, "slice"), kotlinArgs = kotlinArgs).also { it.incrementalCompilation = true; it.classpath = it.classpath + File.pathSeparator + snapDir.path }
            val (c, _) = collect { runCheck(StockPhases.frontend(null), args, listOf(f), services(null, snap)) }
            snap.unpark()
            val a = fullFiles[f]; val b = c.toResult().files.mapKeys { canon(it.key) }[f]
            val dA = fullDeps[f].orEmpty(); val dB = c.depTracker.depPathsByFile.entries.associate { canon(it.key) to it.value.map(::canon).toSet() }[f].orEmpty()
            val d = a?.declarations?.sortedBy { it.fqn } == b?.declarations?.sortedBy { it.fqn }
            val e = a?.expressions == b?.expressions
            val di = a?.diagnostics?.sortedBy { it.toString() } == b?.diagnostics?.sortedBy { it.toString() }
            val l = a?.lambdaSuspendByLineCol == b?.lambdaSuspendByLineCol
            val dp = dA == dB
            if (d && e && di && l && dp) same++
            if (!d) declDiff++; if (!e) exprDiff++; if (!di) diagDiff++; if (!l) lambdaDiff++; if (!dp) depDiff++
            if (!(d && e && di && l && dp)) println("  ${File(f).name}: decl=$d expr=$e (full ${a?.expressions?.size} slice ${b?.expressions?.size}) diag=$di lambda=$l depPaths=$dp (full ${dA.size} slice ${dB.size})")
            if (!e && a != null && b != null) {
                val k = (a.expressions.keys intersect b.expressions.keys).firstOrNull { a.expressions[it] != b.expressions[it] }
                if (k != null) println("      first differing expression $k:\n        full =${a.expressions[k]}\n        slice=${b.expressions[k]}")
            }
        }
        println("## oracle facts slice vs full over ${picks.size} files: identical=$same declDiff=$declDiff exprDiff=$exprDiff diagDiff=$diagDiff lambdaDiff=$lambdaDiff depPathsDiff=$depDiff")
        dir.deleteRecursively()
    }
}
