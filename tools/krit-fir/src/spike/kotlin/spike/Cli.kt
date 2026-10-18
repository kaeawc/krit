package spike

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.FileRef
import java.io.File

/** Baseline: the shipped path (one in-process K2JVMCompiler frontend compile per check) and parity vs the AA runner. */
class Cli(val o: Map<String, String>) {
    val n = (o["files"] ?: "2000").toInt()
    val edits = (o["edits"] ?: "20").toInt()
    val warm = (o["warmup"] ?: "5").toInt()
    val dir = File(o["dir"] ?: "/tmp/spike740/cli$n")

    fun run() {
        dir.deleteRecursively()
        val g = Gen(dir, n); g.generate()
        val idx = DepIndex(); for ((p, t) in g.texts) idx.put(p, t)
        val s = AnalysisSession(listOf(g.libDir.path, g.appDir.path), emptyList())
        val all = g.texts.keys.map { FileRef(it) }
        var t = System.nanoTime()
        val r0 = s.check(0, all, emptySet())
        println("cli cold full check: ${ms(System.nanoTime() - t)} ms findings=${r0.findings.size} errorFiles=${r0.errorFiles.size}")
        val rows = ArrayList<Pair<String, Long>>()
        for (i in 1..(warm + edits)) {
            val e = g.nextEdit()
            val items = HashSet<Pair<String?, String>>()
            for (p in e.changed + e.added + e.deleted) {
                val old = idx.facts[p]
                if (File(p).exists()) idx.put(p, File(p).readText()) else idx.remove(p)
                items += idx.changedItems(old, idx.facts[p])
            }
            val self = (e.changed + e.added + e.deleted).toSet()
            val set = LinkedHashSet<String>().apply { addAll((e.changed + e.added).filter { File(it).exists() }); addAll(idx.dependents(items, self)) }
            t = System.nanoTime()
            val r = s.check(i.toLong(), set.map { FileRef(it) }, emptySet())
            val d = System.nanoTime() - t
            println("cli e$i ${e.kind} set=${set.size} ${ms(d)} ms findings=${r.findings.size}")
            if (i > warm) rows += e.kind to d
        }
        val tot = rows.map { it.second }
        println("## CLI steady-state n=${tot.size} p50=${ms(pct(tot, .5))} p95=${ms(pct(tot, .95))} max=${ms(tot.max())} ms")
        for (k in rows.map { it.first }.distinct().sorted()) {
            val x = rows.filter { it.first == k }.map { it.second }
            println("  %-9s n=%2d p50=%5d max=%5d".format(k, x.size, ms(pct(x, .5)), ms(x.max())))
        }
        if (o["keep"] != "true") dir.deleteRecursively()
    }

    /** Same project state through both runners: compare krit findings per (file, rule) and error-file sets. */
    fun parity() {
        dir.deleteRecursively()
        val g = Gen(dir, n); g.generate()
        val s = AnalysisSession(listOf(g.libDir.path, g.appDir.path), emptyList())
        val specs = listOf(ModSpec("lib", listOf(g.libDir.path)), ModSpec("app", listOf(g.appDir.path), listOf("lib")))
        for (round in 0..(o["edits"] ?: "6").toInt()) {
            if (round > 0) g.nextEdit()
            val cli = s.check(round.toLong(), g.texts.keys.map { FileRef(it) }, emptySet())
            val b = Runner.build(specs)
            val aa = Runner.check(b); b.dispose()
            val cliKrit = cli.findings.groupingBy { it.path to it.rule }.eachCount()
            val aaKrit = HashMap<Pair<String, String>, Int>()
            for ((p, d) in aa) for (k in d.keys) if (k.startsWith("KRIT_RULE|")) {
                val rule = Regex("\\[([A-Za-z0-9_]+)]").find(k)!!.groupValues[1]
                aaKrit.merge(p to rule, 1, Int::plus)
            }
            val cliErr = cli.errorFiles.keys
            val aaErr = aa.filter { it.value.hasError }.keys
            println("round $round: cliFindings=${cli.findings.size} aaFindings=${aaKrit.values.sum()} findingsEqual=${cliKrit == aaKrit} cliErrFiles=${cliErr.size} aaErrFiles=${aaErr.size} errFilesEqual=${cliErr == aaErr}")
            if (cliKrit != aaKrit) println("   only cli: ${(cliKrit.keys - aaKrit.keys).take(3)}  only aa: ${(aaKrit.keys - cliKrit.keys).take(3)}")
            if (cliErr != aaErr) println("   only cli: ${(cliErr - aaErr).take(3).map { File(it).name }}  only aa: ${(aaErr - cliErr).take(3).map { File(it).name }}")
        }
        if (o["keep"] != "true") dir.deleteRecursively()
    }
}
