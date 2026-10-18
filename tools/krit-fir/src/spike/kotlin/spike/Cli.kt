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
        val g = makeProj(o, dir, n, 7L)
        val idx = DepIndex(); for ((p, t) in g.texts) idx.put(p, t)
        val s = AnalysisSession(g.sourceDirs, g.classpath)
        val all = g.texts.keys.map { FileRef(it) }
        var t = System.nanoTime()
        val r0 = s.check(0, all, emptySet())
        println("cli cold full check: ${ms(System.nanoTime() - t)} ms findings=${r0.findings.size} errorFiles=${r0.errorFiles.size}")
        val rows = ArrayList<Pair<String, Long>>()
        for (i in 1..(warm + edits)) {
            val e = g.nextEdit(o["kinds"]?.split(","))
            val items = HashSet<DepIndex.Item>()
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
        val g = makeProj(o, dir, n, 7L)
        val s = AnalysisSession(g.sourceDirs, g.classpath)
        val specs = g.specs(null)
        for (round in 0..(o["edits"] ?: "6").toInt()) {
            if (round > 0) g.nextEdit(null)
            val cli = s.check(round.toLong(), g.texts.keys.map { FileRef(it) }, emptySet())
            val b = Runner.build(specs, g.classpath)
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

    /**
     * Do the oracle facts (declarations, expression types, lambda suspend flags) that krit-fir produces for #810's shared
     * compile come out of an Analysis API session identically? The collector is driven by the same FIR extension
     * through a thread-local registry, so a check of a file in an AA session can fill it.
     */
    fun oracleParity() {
        dir.deleteRecursively()
        val g = makeProj(o, dir, n, 7L)
        val s = AnalysisSession(g.sourceDirs, g.classpath)
        val b = Runner.build(g.specs(null), g.classpath)
        val cc = Class.forName("dev.jasonpearson.krit.fir.oracle.OracleCollector")
        val reg = Class.forName("dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry")
        val inst = reg.getField("INSTANCE").get(null)
        val sample = g.texts.keys.sorted().filterIndexed { i, _ -> i % (g.texts.size / 12 + 1) == 0 }.take(12)
        val coll = cc.getDeclaredConstructor().newInstance()
        reg.getMethod("begin", cc).invoke(inst, coll)
        val t = System.nanoTime()
        Runner.check(b, sample)
        println("AA check with the oracle collector active: ${ms(System.nanoTime() - t)} ms for ${sample.size} files")
        reg.getMethod("end").invoke(inst)
        val aa = cc.getMethod("toResult").invoke(coll) as dev.jasonpearson.krit.fir.oracle.AnalyzeResult
        val cli = s.analyze(sample)
        var same = 0; var diffDecl = 0; var diffExpr = 0; var diffLambda = 0; var missing = 0
        for (p in sample) {
            val a = aa.files[p]; val c = cli.files[p]
            if (a == null || c == null) { missing++; println("  missing payload for ${File(p).name}: aa=${a != null} cli=${c != null}"); continue }
            val d = a.declarations.sortedBy { it.fqn } == c.declarations.sortedBy { it.fqn }; val e = a.expressions == c.expressions; val l = a.lambdaSuspendByLineCol == c.lambdaSuspendByLineCol
            if (d && e && l) same++ else {
                if (!d) diffDecl++; if (!e) diffExpr++; if (!l) diffLambda++
                println("  differs ${File(p).name}: decl=$d (aa=${a.declarations.size}, cli=${c.declarations.size}) expr=$e (aa=${a.expressions.size}, cli=${c.expressions.size}) lambda=$l")
                if (!d) for ((x, y) in a.declarations.zip(c.declarations)) if (x != y) {
                    val sx = x.toString(); val sy = y.toString()
                    var k = 0; while (k < sx.length && k < sy.length && sx[k] == sy[k]) k++
                    println("     first differing class ${x.fqn}: common prefix=$k chars; aa=…${sx.substring(maxOf(0, k - 40)).take(220)}")
                    println("                                           cli=…${sy.substring(maxOf(0, k - 40)).take(220)}")
                    break
                }
                if (!e) { val ka = a.expressions.keys; val kc = c.expressions.keys; println("     expr keys only-aa=${(ka - kc).take(3)} only-cli=${(kc - ka).take(3)} valueDiffs=${ka.intersect(kc).count { a.expressions[it] != c.expressions[it] }}") }
            }
        }
        println("oracle-facts parity over ${sample.size} files: identical=$same declDiff=$diffDecl exprDiff=$diffExpr lambdaDiff=$diffLambda missing=$missing")
        b.dispose()
    }
}
