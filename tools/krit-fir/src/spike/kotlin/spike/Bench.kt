package spike

import java.io.File
import java.lang.management.ManagementFactory
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    val mode = args.firstOrNull() ?: "smoke"
    val opts = args.drop(1).associate { a -> a.removePrefix("--").let { it.substringBefore("=") to it.substringAfter("=", "true") } }
    when (mode) {
        "smoke" -> smoke(opts)
        "inc" -> Inc(opts).run()
        "cli" -> Cli(opts).run()
        "parity" -> Cli(opts).parity()
        "stratc" -> StrategyC(opts).run()
        else -> { System.err.println("unknown mode $mode"); exitProcess(2) }
    }
    exitProcess(0)
}

fun smoke(o: Map<String, String>) {
    val n = (o["files"] ?: "200").toInt()
    val root = File(o["dir"] ?: "/tmp/spike740/smoke$n").also { it.deleteRecursively() }
    val g = Gen(root, n); g.generate()
    val specs = listOf(ModSpec("lib", listOf(g.libDir.path)), ModSpec("app", listOf(g.appDir.path), listOf("lib")))
    val t0 = System.nanoTime()
    val b = Runner.build(specs)
    val t1 = System.nanoTime()
    val r = Runner.check(b)
    val t2 = System.nanoTime()
    println("files=${b.files.size} build=${(t1 - t0) / 1_000_000}ms check=${(t2 - t1) / 1_000_000}ms")
    println("errors=${r.values.sumOf { it.errors }} krit=${r.values.sumOf { it.krit }} errFiles=${r.values.count { it.hasError }}")
    b.dispose()
    root.deleteRecursively()
}

data class Mem(val usedMb: Long, val metaMb: Long, val classes: Int, val threads: Int, val fds: Long)

fun memSnap(gc: Boolean): Mem {
    if (gc) { repeat(2) { System.gc(); Thread.sleep(80) } }
    val rt = Runtime.getRuntime()
    val meta = ManagementFactory.getMemoryPoolMXBeans().firstOrNull { it.name.contains("Metaspace") && !it.name.contains("Compressed") }
    val os = ManagementFactory.getOperatingSystemMXBean()
    val fds = (os as? com.sun.management.UnixOperatingSystemMXBean)?.openFileDescriptorCount ?: -1
    return Mem(
        (rt.totalMemory() - rt.freeMemory()) shr 20, (meta?.usage?.used ?: 0) shr 20,
        ManagementFactory.getClassLoadingMXBean().loadedClassCount, Thread.activeCount(), fds,
    )
}

fun pct(xs: List<Long>, p: Double): Long {
    if (xs.isEmpty()) return 0
    val s = xs.sorted()
    return s[((s.size - 1) * p).toInt().coerceIn(0, s.size - 1)]
}

fun ms(n: Long) = n / 1_000_000

fun cpuNanos(): Long = (ManagementFactory.getOperatingSystemMXBean() as com.sun.management.OperatingSystemMXBean).processCpuTime
fun load1(): String = try { java.io.File("/proc/loadavg").readText().substringBefore(" ") } catch (_: Exception) { "?" }

class Inc(val o: Map<String, String>) {
    val n = (o["files"] ?: "2000").toInt()
    val edits = (o["edits"] ?: "60").toInt()
    val warm = (o["warmup"] ?: "10").toInt()
    val strategy = o["strategy"] ?: "B" // A: fresh everything, B: reuse jdk/stdlib modules, D: B over a dependency slice
    val oracleEvery = (o["oracle"] ?: "1").toInt() // 0 = never
    val gcEvery = (o["gcEvery"] ?: "5").toInt()
    val seed = (o["seed"] ?: "7").toLong()
    val kinds = o["kinds"]?.split(",")
    val dir = File(o["dir"] ?: "/tmp/spike740/inc$n")
    val disposeFirst = o["disposeFirst"] == "true"
    val out = o["out"]?.let { File(it) }
    val coldCheck = o["coldCheck"] != "false" // false: skip the initial all-files check (timing-only runs)
    val oracleSample = (o["oracleSample"] ?: "0").toInt() // 0 = compare every file; N = checked files + N random others

    fun log(s: String) { println(s); out?.appendText(s + "\n") }

    fun specsFor(g: Gen, slice: Set<String>?): List<ModSpec> {
        if (slice == null) return listOf(ModSpec("lib", listOf(g.libDir.path)), ModSpec("app", listOf(g.appDir.path), listOf("lib")))
        val lib = slice.filter { it.startsWith(g.libDir.path) }.sorted()
        val app = slice.filter { it.startsWith(g.appDir.path) }.sorted()
        val l = ArrayList<ModSpec>()
        if (lib.isNotEmpty()) l += ModSpec("lib", emptyList(), files = lib)
        if (app.isNotEmpty()) l += ModSpec("app", emptyList(), if (lib.isNotEmpty()) listOf("lib") else emptyList(), files = app)
        return l
    }

    fun run() {
        dir.deleteRecursively()
        val g = Gen(dir, n, seed); g.generate()
        log("# inc strategy=$strategy files=$n edits=$edits warmup=$warm oracle=$oracleEvery heapMax=${Runtime.getRuntime().maxMemory() shr 20}MB")
        val idx = DepIndex()
        var t = System.nanoTime()
        for ((p, txt) in g.texts) idx.put(p, txt)
        log("depindex build: ${ms(System.nanoTime() - t)} ms for ${g.texts.size} files")

        // cold: full session + full check
        t = System.nanoTime()
        var cur = Runner.build(specsFor(g, null))
        val tb = System.nanoTime()
        val cache = HashMap(if (coldCheck) Runner.check(cur) else emptyMap())
        val tc = System.nanoTime()
        var libs = cur.libs
        log("cold: build=${ms(tb - t)}ms checkAll=${ms(tc - tb)}ms files=${cur.files.size} errors=${cache.values.sumOf { it.errors }} krit=${cache.values.sumOf { it.krit }}")
        log("mem after cold: ${memSnap(true)}")

        data class Row(val i: Int, val kind: String, val scan: Long, val build: Long, val check: Long, val dispose: Long, val checked: Int, val mism: Int, val mem: Mem?, val cpu: Long, val load: String)
        val rows = ArrayList<Row>()
        for (i in 1..(warm + edits)) {
            val e = g.nextEdit(kinds)
            val total0 = System.nanoTime()
            val cpu0 = cpuNanos()
            // --- scan: re-read changed files, update index, derive check set
            val changedNow = (e.changed + e.added).filter { File(it).exists() }
            val items = HashSet<Pair<String?, String>>()
            for (p in e.changed + e.added + e.deleted) {
                val old = idx.facts[p]
                val txtNew = if (File(p).exists()) File(p).readText() else null
                if (txtNew != null) idx.put(p, txtNew) else idx.remove(p)
                items += idx.changedItems(old, idx.facts[p])
            }
            val self = (e.changed + e.added + e.deleted).toSet()
            val deps = idx.dependents(items, self)
            val checkSet = LinkedHashSet<String>().apply { addAll(changedNow); addAll(deps) }
            val tScan = System.nanoTime() - total0
            // --- build
            val old = cur
            if (disposeFirst) old.dispose()
            val slice = if (strategy == "D") idx.forwardClosure(checkSet) else null
            val tb0 = System.nanoTime()
            val fresh = Runner.build(specsFor(g, slice), reuse = if (strategy == "A") null else libs)
            val tBuild = System.nanoTime() - tb0
            // --- check
            val tc0 = System.nanoTime()
            val res = Runner.check(fresh, checkSet)
            val tCheck = System.nanoTime() - tc0
            for (d in e.deleted) cache.remove(d)
            cache.putAll(res)
            val td0 = System.nanoTime()
            if (!disposeFirst) old.dispose()
            val tDisp = System.nanoTime() - td0
            val cpuUsed = cpuNanos() - cpu0
            cur = fresh
            if (strategy != "A") libs = libs ?: fresh.libs

            val mem = if (i % gcEvery == 0) memSnap(true) else null
            var mism = -1
            if (oracleEvery > 0 && i % oracleEvery == 0) {
                val fullB = Runner.build(specsFor(g, null))
                val sample: Collection<String>? = if (oracleSample > 0) {
                    val pool = fullB.files.keys.toList()
                    (checkSet + (1..oracleSample).map { pool[g.rnd.nextInt(pool.size)] } + e.deleted).toSet()
                } else null
                val full = Runner.check(fullB, sample)
                fullB.dispose()
                mism = 0
                val bad = ArrayList<String>()
                for (p in sample ?: (full.keys + cache.keys)) {
                    val a = full[p]?.keys; val b = cache[p]?.keys
                    if (b == null && sample != null && p !in checkSet) continue // never cached (sampled, not rechecked): no claim
                    if (a != b) { mism++; if (bad.size < 3) bad += "${File(p).name}: want=${a?.size} got=${b?.size} ${(a.orEmpty() - b.orEmpty()).take(1)} / ${(b.orEmpty() - a.orEmpty()).take(1)}" }
                }
                if (mism > 0) log("  MISMATCH edit#$i ${e.kind}: $mism files; e.g. $bad")
            }
            val r = Row(i, e.kind, tScan, tBuild, tCheck, tDisp, checkSet.size, mism, mem, cpuUsed, load1())
            rows += r
            log("e$i ${e.kind} set=${checkSet.size} (deps=${deps.size}) scan=${ms(tScan)} build=${ms(tBuild)} check=${ms(tCheck)} dispose=${ms(tDisp)} total=${ms(tScan + tBuild + tCheck)}ms cpu=${ms(cpuUsed)}ms load=${load1()}${if (mem != null) " mem=$mem" else ""}${if (mism >= 0) " mism=$mism" else ""}")
        }
        val meas = rows.filter { it.i > warm }
        fun stat(name: String, rs: List<Row>) {
            if (rs.isEmpty()) return
            val tot = rs.map { it.scan + it.build + it.check }
            log("%-9s n=%3d total p50=%5d p95=%5d max=%5d ms | cpu p50=%5d p95=%5d | build p50=%5d | check p50=%5d p95=%5d | set p50=%d | dispose p50=%d".format(
                name, rs.size, ms(pct(tot, .5)), ms(pct(tot, .95)), ms(tot.max()), ms(pct(rs.map { it.cpu }, .5)), ms(pct(rs.map { it.cpu }, .95)),
                ms(pct(rs.map { it.build }, .5)), ms(pct(rs.map { it.check }, .5)), ms(pct(rs.map { it.check }, .95)),
                pct(rs.map { it.checked.toLong() }, .5), ms(pct(rs.map { it.dispose }, .5))))
        }
        log("## steady-state (edits > $warm), strategy=$strategy files=$n")
        stat("ALL", meas)
        for (k in meas.map { it.kind }.distinct().sorted()) stat(k, meas.filter { it.kind == k })
        stat("firstHalf", meas.take(meas.size / 2)); stat("lastHalf", meas.drop(meas.size / 2))
        log("load1 during measured edits: min=${meas.minOf { it.load }} max=${meas.maxOf { it.load }}")
        val mems = rows.mapNotNull { r -> r.mem?.let { r.i to it } }
        log("## heap after GC / metaspace / classes / threads / fds by edit:")
        for ((i, m) in mems) log("  edit $i: used=${m.usedMb}MB meta=${m.metaMb}MB classes=${m.classes} threads=${m.threads} fds=${m.fds}")
        val mmis = rows.filter { it.mism >= 0 }
        log("## correctness: oracle ran on ${mmis.size} edits, mismatching edits=${mmis.count { it.mism > 0 }} total mismatching files=${mmis.sumOf { it.mism }}")
        for (r in mmis.filter { it.mism > 0 }) log("   edit ${r.i} (${r.kind}) mism=${r.mism} set=${r.checked}")
        cur.dispose()
        if (o["keep"] != "true") dir.deleteRecursively()
    }
}
