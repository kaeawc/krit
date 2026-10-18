package spike

import java.io.File
import org.jetbrains.kotlin.analysis.api.components.KaDiagnosticCheckerFilter
import java.lang.management.ManagementFactory
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    val mode = args.firstOrNull() ?: "smoke"
    val opts = args.drop(1).associate { a -> a.removePrefix("--").let { it.substringBefore("=") to it.substringAfter("=", "true") } }
    // Rule tiers: --excludeRules=A,B runs every other FIR rule (the enabled set is bound when each FIR session is built).
    opts["excludeRules"]?.let { ex ->
        val skip = ex.split(",").toSet()
        dev.jasonpearson.krit.fir.FirRuleContext.begin(dev.jasonpearson.krit.fir.FirRuleCompileContext(
            enabledRuleIds = dev.jasonpearson.krit.fir.FirRuleDiscovery.rules.map { it.ruleId }.toSet() - skip))
    }
    try {
        when (mode) {
            "smoke" -> smoke(opts)
            "inc" -> Inc(opts).run()
            "cli" -> Cli(opts).run()
            "parity" -> Cli(opts).parity()
            "oracle" -> Cli(opts).oracleParity()
            "surf" -> {
                val g = Gen(File("/tmp/spike740/surf"), 60); g.generate()
                val b = Runner.build(g.specs(null))
                println(Runner.surfaceText(b.files.getValue(g.libPath(5))))
                println(Runner.surfaceText(b.files.getValue(g.appPath(7))))
                b.dispose(); File("/tmp/spike740/surf").deleteRecursively()
            }
            "ctx" -> ctxProbe()
            "dangling" -> Dangling(opts).run()
            "fidelity" -> Dangling(opts).fidelity()
            "filters" -> filtersProbe(opts)
            "depdebug" -> depDebug(opts)
            "stratc" -> StrategyC(opts).run()
            else -> { System.err.println("unknown mode $mode"); exitProcess(2) }
        }
    } catch (t: Throwable) {
        t.printStackTrace()
        exitProcess(1)
    }
    exitProcess(0)
}

fun depDebug(opts: Map<String, String>) {
    val n = (opts["files"] ?: "10000").toInt()
    val g = Gen(File("/tmp/spike740/dd"), n); g.generate()
    val idx = DepIndex(); for ((p, t) in g.texts) idx.put(p, t)
    for (i in listOf(5, 123, 777, 1500, 2400)) {
        val p = g.libPath(i); val old = idx.facts[p]
        g.sigToggle(i)
        val items = idx.changedItems(old, DepIndex.analyze(g.texts[p]!!))
        println("lib $i items=$items deps=${idx.dependents(items, setOf(p)).size}")
    }
    File("/tmp/spike740/dd").deleteRecursively()
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
    val oracleFullEvery = (o["oracleFullEvery"] ?: "0").toInt() // with oracleSample: also do a full compare every N edits
    val oracleSample = (o["oracleSample"] ?: "0").toInt() // 0 = compare every file; N = checked files + N random others

    fun log(s: String) { println(s); out?.appendText(s + "\n") }

    fun run() {
        dir.deleteRecursively()
        val g = makeProj(o, dir, n, seed)
        log("# ${g.describe()}")
        log("# inc strategy=$strategy files=$n edits=$edits warmup=$warm oracle=$oracleEvery heapMax=${Runtime.getRuntime().maxMemory() shr 20}MB")
        val idx = DepIndex().also { it.scopeAware = o["scopeAware"] != "false" }
        var t = System.nanoTime()
        for ((p, txt) in g.texts) idx.put(p, txt)
        log("depindex build: ${ms(System.nanoTime() - t)} ms for ${g.texts.size} files")

        // cold: full session + full check
        t = System.nanoTime()
        var cur = Runner.build(g.specs(null), g.classpath)
        val tb = System.nanoTime()
        val cache = HashMap(if (coldCheck) Runner.check(cur) else emptyMap())
        var prevFull: Map<String, FileDiags>? = if (coldCheck) HashMap(cache) else null
        val causes = java.util.TreeMap<String, Int>() // strict-mismatch diagnostic factories (file x edit)
        val precision = ArrayList<Triple<String, Int, Pair<Int, Int>>>() // kind, deps size, (truth size, missed)
        val tc = System.nanoTime()
        var libs = cur.libs
        log("cold: build=${ms(tb - t)}ms checkAll=${ms(tc - tb)}ms files=${cur.files.size} errors=${cache.values.sumOf { it.errors }} krit=${cache.values.sumOf { it.krit }}")
        log("mem after cold: ${memSnap(true)}")

        data class Row(val i: Int, val kind: String, val scan: Long, val build: Long, val check: Long, val dispose: Long, val checked: Int, val mism: Int, val mem: Mem?, val cpu: Long, val load: String, val vmism: Int = -1)
        val rows = ArrayList<Row>()
        val shadow = HashMap(g.texts) // texts as the daemon last saw them (for priming old surfaces)
        val surf = HashMap<String, String>() // resolved declaration surface per file, as of its last check
        for (i in 1..(warm + edits)) {
            val e = g.nextEdit(kinds)
            // Strategy S: the daemon would hold the previous surface of an edited file from its last check; emulate that
            // (untimed) by resolving the file's pre-edit text.
            if (strategy == "S") for (p in e.changed) if (p !in surf && shadow.containsKey(p)) {
                val slice0 = idx.forwardClosure(setOf(p)) + p
                val pb = Runner.build(g.specs(slice0), g.classpath, reuse = libs, overlay = mapOf(p to shadow.getValue(p)))
                surf[p] = Runner.surface(pb.files.getValue(p)); pb.dispose()
            }
            val total0 = System.nanoTime()
            val cpu0 = cpuNanos()
            // --- scan: re-read changed files, update index, derive check set
            val changedNow = (e.changed + e.added).filter { File(it).exists() }
            val items = HashSet<DepIndex.Item>()
            val oldFacts = HashMap<String, DepIndex.Facts?>()
            for (p in e.changed + e.added + e.deleted) {
                val old = idx.facts[p]
                oldFacts[p] = old
                val txtNew = if (File(p).exists()) File(p).readText() else null
                if (txtNew != null) idx.put(p, txtNew) else idx.remove(p)
                items += idx.changedItems(old, idx.facts[p])
            }
            val self = (e.changed + e.added + e.deleted).toSet()
            val deps = idx.dependents(items, self)
            val checkSet = LinkedHashSet<String>().apply { addAll(changedNow); addAll(deps) }
            val tScan = System.nanoTime() - total0
            var checkSetFinal: Set<String> = checkSet
            var depsFinal: Set<String> = deps
            val old = cur
            var fresh: Built
            var tBuild: Long
            var tCheck: Long
            var res: Map<String, FileDiags>
            if (strategy == "S") {
                // Surface-gated fixpoint: check the edited files; only when a file's resolved surface changed, check its
                // lexical dependents, and repeat for those (replaces the "inferred type" heuristic closure).
                val itemsAD = HashSet<DepIndex.Item>()
                val itemsByFile = HashMap<String, Set<DepIndex.Item>>()
                for (p in e.changed + e.added + e.deleted) {
                    val before = oldFacts[p]
                    val its = idx.changedItems(before, idx.facts[p])
                    itemsByFile[p] = its
                    if (p in e.added || p in e.deleted) itemsAD += its
                }
                val edited = changedNow.toSet()
                val queue = LinkedHashSet<String>(changedNow)
                queue += idx.dependents(itemsAD, self)
                val done = LinkedHashSet<String>()
                val merged = HashMap<String, FileDiags>()
                var tb = 0L; var tc = 0L; var last: Built? = null; var waves = 0
                while (queue.isNotEmpty() && waves < 16) {
                    val files = queue.filter { it !in done && File(it).exists() }.toSet(); queue.clear()
                    if (files.isEmpty()) break
                    val slice = idx.forwardClosure(files)
                    val t1 = System.nanoTime()
                    val wb = Runner.build(g.specs(slice), g.classpath, reuse = libs)
                    tb += System.nanoTime() - t1
                    val t2 = System.nanoTime()
                    merged.putAll(Runner.check(wb, files))
                    val newSurf = files.associateWith { Runner.surface(wb.files.getValue(it)) }
                    tc += System.nanoTime() - t2
                    done += files
                    for (f in files) {
                        val prev = surf[f]
                        val changed = when {
                            prev != null -> prev != newSurf.getValue(f)
                            f in edited -> true
                            else -> idx.facts[f]?.decls?.any { it.inferred } == true // unknown history: conservative fallback
                        }
                        surf[f] = newSurf.getValue(f)
                        if (changed) {
                            val its = itemsByFile[f]?.takeIf { it.isNotEmpty() } ?: idx.facts[f]?.decls.orEmpty().map { DepIndex.Item(if (it.owner == null) idx.facts[f]?.pkg else null, it.owner, it.name) }.toSet()
                            queue += idx.dependents(its, done + files)
                        }
                    }
                    last?.dispose(); last = wb; waves++
                }
                fresh = last ?: Runner.build(g.specs(emptySet()), g.classpath)
                tBuild = tb; tCheck = tc; res = merged
                checkSetFinal = done; depsFinal = done - edited
            } else {
            // --- build
            if (disposeFirst) old.dispose()
            val slice = if (strategy == "D") idx.forwardClosure(checkSet) else null
            val tb0 = System.nanoTime()
            fresh = Runner.build(g.specs(slice), g.classpath, reuse = if (strategy == "A") null else libs)
            tBuild = System.nanoTime() - tb0
            // --- check
            val tc0 = System.nanoTime()
            res = Runner.check(fresh, checkSet)
            tCheck = System.nanoTime() - tc0
            }
            for (d in e.deleted) cache.remove(d)
            cache.putAll(res)
            val td0 = System.nanoTime()
            if (!disposeFirst && old !== fresh) old.dispose()
            val tDisp = System.nanoTime() - td0
            val cpuUsed = cpuNanos() - cpu0
            cur = fresh
            for (p in e.changed + e.added) g.texts[p]?.let { shadow[p] = it }
            for (p in e.deleted) { shadow.remove(p); surf.remove(p) }
            if (strategy != "A") libs = libs ?: fresh.libs

            val mem = if (i % gcEvery == 0) memSnap(true) else null
            var mism = -1
            var vmism = -1
            if (oracleEvery > 0 && i % oracleEvery == 0) {
                val fullB = Runner.build(g.specs(null), g.classpath)
                val fullNow = oracleSample == 0 || (oracleFullEvery > 0 && i % oracleFullEvery == 0)
                val sample: Collection<String>? = if (!fullNow) {
                    val pool = fullB.files.keys.toList()
                    (checkSetFinal + (1..oracleSample).map { pool[g.rnd.nextInt(pool.size)] } + e.deleted).toSet()
                } else null
                val full = Runner.check(fullB, sample)
                fullB.dispose()
                mism = 0; vmism = 0
                if (fullNow && prevFull != null) {
                    val pf = prevFull!!
                    val truth = (full.keys + pf.keys).filter { p -> p !in self && full[p]?.keys != pf[p]?.keys }.toSet()
                    val missed = truth - depsFinal
                    precision += Triple(e.kind, depsFinal.size, truth.size to missed.size)
                    log("  truth: files (other than the edited) whose diagnostics changed=${truth.size}, flagged dependents=${depsFinal.size}, missed=${missed.size}")
                }
                if (fullNow) prevFull = full
                val bad = ArrayList<String>()
                for (p in sample ?: (full.keys + cache.keys)) {
                    val a = full[p]?.keys; val b = cache[p]?.keys
                    if (b == null && sample != null && p !in checkSetFinal) continue // never cached (sampled, not rechecked): no claim
                    fun verdict(d: FileDiags?) = d?.let { x -> x.keys.filter { it.startsWith("KRIT_RULE|") || it.startsWith("RULEERR|") }.toSet() to x.hasError }
                    if (verdict(full[p]) != verdict(cache[p])) { vmism++; log("  VERDICT-MISMATCH edit#$i ${e.kind} ${File(p).name}") }
                    if (a != b) { for (k in (a.orEmpty() - b.orEmpty()) + (b.orEmpty() - a.orEmpty())) causes.merge(k.substringBefore('|'), 1, Int::plus); mism++; if (bad.size < 3) bad += "${File(p).name}: want=${a?.size} got=${b?.size} ${(a.orEmpty() - b.orEmpty()).take(1)} / ${(b.orEmpty() - a.orEmpty()).take(1)}" }
                }
                if (mism > 0) log("  MISMATCH(strict) edit#$i ${e.kind}: $mism files, verdict-level $vmism; e.g. $bad")
            }
            val r = Row(i, e.kind, tScan, tBuild, tCheck, tDisp, checkSetFinal.size, mism, mem, cpuUsed, load1(), vmism)
            rows += r
            log("e$i ${e.kind} set=${checkSetFinal.size} (deps=${depsFinal.size}) scan=${ms(tScan)} build=${ms(tBuild)} check=${ms(tCheck)} dispose=${ms(tDisp)} total=${ms(tScan + tBuild + tCheck)}ms cpu=${ms(cpuUsed)}ms load=${load1()}${if (mem != null) " mem=$mem" else ""}${if (mism >= 0) " mism=$mism" else ""}")
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
        if (precision.isNotEmpty()) {
            log("## dependents precision/recall vs ground truth (edits with a full oracle): n=${precision.size}")
            for (k in precision.map { it.first }.distinct().sorted()) {
                val x = precision.filter { it.first == k }
                log("  %-9s n=%2d flagged avg=%.1f  truly-changed avg=%.1f  missed total=%d".format(k, x.size, x.map { it.second }.average(), x.map { it.third.first }.average(), x.sumOf { it.third.second }))
            }
        }
        val mems = rows.mapNotNull { r -> r.mem?.let { r.i to it } }
        log("## heap after GC / metaspace / classes / threads / fds by edit:")
        for ((i, m) in mems) log("  edit $i: used=${m.usedMb}MB meta=${m.metaMb}MB classes=${m.classes} threads=${m.threads} fds=${m.fds}")
        val mmis = rows.filter { it.mism >= 0 }
        log("## correctness: oracle ran on ${mmis.size} edits; strict(all diagnostics) mismatching edits=${mmis.count { it.mism > 0 }} files=${mmis.sumOf { it.mism }}; verdict(krit findings + error gating) mismatching edits=${mmis.count { it.vmism > 0 }} files=${mmis.sumOf { it.vmism }}")
        log("## strict-mismatch causes by diagnostic factory (file x edit): $causes")
        for (r in mmis.filter { it.mism > 0 }) log("   edit ${r.i} (${r.kind}) strict=${r.mism} verdict=${r.vmism} set=${r.checked}")
        log("## process memory at end: " + (try { File("/proc/self/status").readLines().filter { it.startsWith("VmHWM") || it.startsWith("VmRSS") }.joinToString(" ") { it.replace(Regex("\\s+"), " ") } } catch (_: Exception) { "n/a" }) +
            " gcPeakHeap(MB)=" + ManagementFactory.getMemoryPoolMXBeans().filter { it.type == java.lang.management.MemoryType.HEAP }.sumOf { it.peakUsage.used } / (1 shl 20))
        cur.dispose()
        if (o["keep"] != "true") dir.deleteRecursively()
    }
}

/** Do per-request rule context (thread-local FirRuleContext) and rule selection reach checkers running inside Analysis API sessions? */
fun ctxProbe() {
    val g = Gen(File("/tmp/spike740/ctx"), 120); g.generate()
    val noisy = g.appPath(25) // App25 calls println
    fun krit(b: Built, p: String) = Runner.checkFile(b.files.getValue(p)).keys.filter { it.startsWith("KRIT_RULE") }.map { Regex("\\[([A-Za-z0-9_]+)]").find(it)?.groupValues?.get(1) }
    val specs = g.specs(null)
    println("-- no context at session build or check: ${krit(Runner.build(specs).also { }, noisy)}")
    // context set at check time only (session built without it)
    run {
        val b = Runner.build(specs)
        dev.jasonpearson.krit.fir.FirRuleContext.begin(dev.jasonpearson.krit.fir.FirRuleCompileContext(testFiles = setOf(noisy), files = setOf(noisy)))
        println("-- testFiles={noisy} set at CHECK time only (session built earlier): ${krit(b, noisy)}  (empty => context reached the checker)")
        dev.jasonpearson.krit.fir.FirRuleContext.end(); b.dispose()
    }
    // context set at build time and check time
    run {
        dev.jasonpearson.krit.fir.FirRuleContext.begin(dev.jasonpearson.krit.fir.FirRuleCompileContext(testFiles = setOf(noisy), files = setOf(noisy)))
        val b = Runner.build(specs)
        println("-- testFiles={noisy} set at BUILD and CHECK time: ${krit(b, noisy)}")
        dev.jasonpearson.krit.fir.FirRuleContext.end(); b.dispose()
    }
    // rule selection: enabled set captured when the FIR session (KritFirCheckers) is constructed
    run {
        dev.jasonpearson.krit.fir.FirRuleContext.begin(dev.jasonpearson.krit.fir.FirRuleCompileContext(enabledRuleIds = setOf("PrintStackTrace")))
        val b = Runner.build(specs)
        println("-- enabledRuleIds={PrintStackTrace} at build+first check: ${krit(b, noisy)} (PrintlnInProduction must be absent)")
        dev.jasonpearson.krit.fir.FirRuleContext.end()
        println("-- same session rechecked with no context: ${krit(b, noisy)}")
        b.dispose()
    }
    // which thread runs checkers?
    run {
        val b = Runner.build(specs)
        val t = Thread.currentThread().name
        println("-- caller thread=$t; checkers ran inline: ${Thread.getAllStackTraces().keys.map { it.name }.filter { it.contains("ForkJoin") || it.contains("pool") }.size} pool threads exist")
        b.dispose()
    }
    File("/tmp/spike740/ctx").deleteRecursively()
}

/** Checker filter choice: the shipped compile reports the compiler's common diagnostics only; compare AA filters on cost and findings. */
fun filtersProbe(o: Map<String, String>) {
    val n = (o["files"] ?: "300").toInt()
    val g = Gen(File("/tmp/spike740/flt"), n); g.generate()
    val b = Runner.build(g.specs(null))
    val paths = b.files.keys.sorted().take(200)
    val results = HashMap<String, Map<String, FileDiags>>()
    for (round in 1..3) for (f in listOf(KaDiagnosticCheckerFilter.ONLY_COMMON_CHECKERS, KaDiagnosticCheckerFilter.EXTENDED_AND_COMMON_CHECKERS, KaDiagnosticCheckerFilter.ONLY_EXTENDED_CHECKERS)) {
        Runner.filter = f
        val bb = Runner.build(g.specs(null))
        val t = System.nanoTime(); val cpu0 = cpuNanos()
        val r = Runner.check(bb, paths)
        println("round $round $f: ${ms(System.nanoTime() - t)} ms wall, ${ms(cpuNanos() - cpu0)} ms cpu for ${paths.size} files; krit findings=${r.values.sumOf { it.krit }} total diags=${r.values.sumOf { it.keys.size }} errors=${r.values.sumOf { it.errors }}")
        results[f.name] = r; bb.dispose()
    }
    val a = results["ONLY_COMMON_CHECKERS"]!!; val c = results["EXTENDED_AND_COMMON_CHECKERS"]!!
    println("krit findings equal between ONLY_COMMON and EXTENDED_AND_COMMON: ${a.mapValues { it.value.keys.filter { k -> k.startsWith("KRIT") } } == c.mapValues { it.value.keys.filter { k -> k.startsWith("KRIT") } }}")
    b.dispose(); File("/tmp/spike740/flt").deleteRecursively()
}
