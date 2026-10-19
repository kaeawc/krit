package spike

import java.io.File
import java.lang.management.ManagementFactory
import kotlin.system.exitProcess

val SCRATCH: File = File(System.getProperty("spike.scratch") ?: "/tmp/claude-0/-home-user-krit/8d3b130e-8c6f-5030-b03b-be3af5a37e79/scratchpad/f740/work")

fun main(args: Array<String>) {
    val mode = args.firstOrNull() ?: "smoke"
    val opts = args.drop(1).associate { a -> a.removePrefix("--").let { it.substringBefore("=") to it.substringAfter("=", "true") } }
    try {
        when (mode) {
            "smoke" -> smoke(opts)
            "inc" -> Inc(opts).run()
            "snapcost" -> SnapCost(opts).run()
            "shapes" -> Shapes(opts).run()
            "bta" -> Bta(opts).run()
            "inplace" -> InPlace(opts).run()
            "rulediff" -> RuleDiff(opts).run()
            "oraclefacts" -> OracleFacts(opts).run()
            "leak" -> LeakFinder(opts).run()
            else -> { System.err.println("unknown mode $mode"); exitProcess(2) }
        }
    } catch (t: Throwable) {
        t.printStackTrace()
        exitProcess(1)
    }
    exitProcess(0)
}

fun smoke(o: Map<String, String>) {
    val n = (o["files"] ?: "200").toInt()
    val root = File(SCRATCH, "smoke$n").also { it.deleteRecursively() }
    val g = Gen(root, n); g.generate()
    val files = g.texts.keys.toList()
    val out = File(root, "out").also { it.mkdirs() }
    var t = System.nanoTime()
    val shipped = runShipped(kritArgs(files, g.classpath, out), files, frontendOnly = true)
    println("shipped frontend-only: ${ms(System.nanoTime() - t)} ms exit=${shipped.exit} findings=${shipped.verdicts.values.sumOf { it.findings.size }} errFiles=${shipped.verdicts.values.count { it.error }}")
    val st = WarmState(reuseEnv = o["reuseEnv"] != "false", reuseLibs = o["reuseLibs"] != "false")
    repeat(3) { i ->
        val tm = Timings()
        t = System.nanoTime()
        val warm = runCheck(Phases.warmFrontend(st, null, tm), kritArgs(files, g.classpath, out), files)
        println("warm #$i: ${ms(System.nanoTime() - t)} ms [$tm] exit=${warm.exit} findings=${warm.verdicts.values.sumOf { it.findings.size }} errFiles=${warm.verdicts.values.count { it.error }} equal=${Compare.verdictDiff(shipped.verdicts, warm.verdicts).isEmpty()} strictEqual=${Compare.strictDiff(shipped.verdicts, warm.verdicts).isEmpty()}")
        Compare.strictDiff(shipped.verdicts, warm.verdicts).take(3).forEach { println("   " + Compare.explain(it, shipped.verdicts[it], warm.verdicts[it])) }
    }
    st.dispose()
    root.deleteRecursively()
}

object Compare {
    /** Files whose Go-visible verdict (findings, error gate, rule-error set) differ. */
    fun verdictDiff(a: Map<String, FileVerdict>, b: Map<String, FileVerdict>, files: Collection<String> = a.keys + b.keys): List<String> =
        files.filter { a[it]?.verdictKey() != b[it]?.verdictKey() }

    fun strictDiff(a: Map<String, FileVerdict>, b: Map<String, FileVerdict>, files: Collection<String> = a.keys + b.keys): List<String> =
        files.filter { a[it] != b[it] }

    fun explain(p: String, a: FileVerdict?, b: FileVerdict?): String {
        if (a == null || b == null) return "${File(p).name}: missing a=${a != null} b=${b != null}"
        val sb = StringBuilder(File(p).name)
        if (a.findings != b.findings) sb.append(" findings only-ref=${(a.findings - b.findings.toSet()).take(2)} only-inc=${(b.findings - a.findings.toSet()).take(2)}")
        if (a.error != b.error) sb.append(" error ref=${a.error} inc=${b.error}")
        if (a.ruleErrors.keys != b.ruleErrors.keys) sb.append(" ruleErrors ref=${a.ruleErrors.keys} inc=${b.ruleErrors.keys}")
        if (a.diags != b.diags) sb.append(" diags only-ref=${(a.diags - b.diags.toSet()).take(2).map { it.take(160) }} only-inc=${(b.diags - a.diags.toSet()).take(2).map { it.take(160) }}")
        return sb.toString()
    }
}

data class Mem(val heapMb: Long, val metaMb: Long, val classes: Int, val threads: Int, val fds: Long, val rssMb: Long, val hwmMb: Long) {
    override fun toString() = "heap=${heapMb}MB meta=${metaMb}MB classes=$classes threads=$threads fds=$fds rss=${rssMb}MB hwm=${hwmMb}MB"
}

fun memSnap(gc: Boolean): Mem {
    if (gc) repeat(2) { System.gc(); Thread.sleep(80) }
    val rt = Runtime.getRuntime()
    val meta = ManagementFactory.getMemoryPoolMXBeans().firstOrNull { it.name.contains("Metaspace") && !it.name.contains("Compressed") }
    val os = ManagementFactory.getOperatingSystemMXBean()
    val fds = (os as? com.sun.management.UnixOperatingSystemMXBean)?.openFileDescriptorCount ?: -1
    fun status(k: String): Long = try {
        File("/proc/self/status").readLines().first { it.startsWith(k) }.split(Regex("\\s+"))[1].toLong() / 1024
    } catch (_: Exception) { -1 }
    return Mem(
        (rt.totalMemory() - rt.freeMemory()) shr 20, (meta?.usage?.used ?: 0) shr 20,
        ManagementFactory.getClassLoadingMXBean().loadedClassCount, Thread.activeCount(), fds, status("VmRSS"), status("VmHWM"),
    )
}

fun pct(xs: List<Long>, p: Double): Long {
    if (xs.isEmpty()) return 0
    val s = xs.sorted()
    return s[((s.size - 1) * p).toInt().coerceIn(0, s.size - 1)]
}

fun ms(n: Long) = n / 1_000_000
fun cpuNanos(): Long = (ManagementFactory.getOperatingSystemMXBean() as com.sun.management.OperatingSystemMXBean).processCpuTime
fun load1(): String = try { File("/proc/loadavg").readText().substringBefore(" ") } catch (_: Exception) { "?" }

/** Live-object class histogram of this JVM (jcmd GC.class_histogram; forces a full GC). */
fun histogram(top: Int): List<String> = try {
    val p = ProcessBuilder("jcmd", ProcessHandle.current().pid().toString(), "GC.class_histogram").redirectErrorStream(true).start()
    p.inputStream.bufferedReader().readLines().drop(1).take(top + 2)
} catch (e: Exception) { listOf("histogram unavailable: $e") }
