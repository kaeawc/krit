package spike

import java.io.File

/**
 * What a binary snapshot costs relative to the check krit runs today (frontend only), per compile kind, in one JVM:
 *  - fe:      shipped FrontendOnlyJvmPipeline (today's check)
 *  - full:    K2JVMCompiler, frontend + fir2ir + JVM backend + class files (+ krit checkers)
 *  - header:  K2JVMCompiler -Xheader-mode (skeleton classes, inline bodies kept; checkers are skipped in this mode)
 *  - abigen:  full + the jvm-abi-gen plugin writing an ABI class directory
 *  - warmfe / warmfull: the same through WarmFrontendPhase (env + libraries reused)
 * Each kind runs --reps times after one discarded run; p50 reported. Also reports output sizes.
 */
class SnapCost(val o: Map<String, String>) {
    val n = (o["files"] ?: "2000").toInt()
    val reps = (o["reps"] ?: "3").toInt()
    val kindsToRun = (o["kinds"] ?: "fe,full,header,abigen,warmfe,warmfull").split(",")
    val kotlinArgs = o["kotlinArgs"]?.split(",")?.filter { it.isNotBlank() } ?: emptyList()
    val dir = File(SCRATCH, o["dir"] ?: "snapcost-$n")

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        val g = makeProj(o, File(dir, "src"), n, 7L)
        val files = g.texts.keys.map(::canon)
        println("# ${g.describe()} load=${load1()}")
        val st = WarmState()
        val abigen = System.getProperty("spike.abigen")
        for (kind in kindsToRun) {
            val times = ArrayList<Long>(); val cpus = ArrayList<Long>()
            var info = ""
            repeat(reps + 1) { rep ->
                val out = File(dir, "out-$kind").also { it.deleteRecursively(); it.mkdirs() }
                val args = kritArgs(files, g.classpath, out, kotlinArgs = kotlinArgs)
                if (kind == "header") args.headerMode = true
                if (kind == "abigen") {
                    args.pluginClasspaths = args.pluginClasspaths!! + abigen
                    args.pluginOptions = arrayOf("plugin:org.jetbrains.kotlin.jvm.abi:outputDir=${File(dir, "abi").also { it.deleteRecursively() }.path}")
                }
                val t0 = System.nanoTime(); val c0 = cpuNanos()
                val r = when (kind) {
                    "fe" -> runShipped(args, files, frontendOnly = true)
                    "full", "header", "abigen" -> runShipped(args, files, frontendOnly = false)
                    "warmfe" -> runCheck(Phases.warmFrontend(st, null, Timings()), args, files)
                    "warmfull" -> runCheck(Phases.warmFull(st, null, Timings()), args, files)
                    else -> error(kind)
                }
                val d = System.nanoTime() - t0; val c = cpuNanos() - c0
                if (rep > 0) { times += d; cpus += c }
                val classes = out.walkTopDown().count { it.isFile && it.name.endsWith(".class") }
                val bytes = out.walkTopDown().filter { it.isFile }.sumOf { it.length() }
                val abi = File(dir, "abi").walkTopDown().count { it.isFile && it.name.endsWith(".class") }
                info = "exit=${r.exit} findings=${r.verdicts.values.sumOf { it.findings.size }} errFiles=${r.verdicts.values.count { it.error }} classes=$classes bytes=${bytes / 1024}KB" + if (kind == "abigen") " abiClasses=$abi" else ""
                println("  $kind rep$rep ${ms(d)} ms cpu=${ms(c)} $info load=${load1()}")
            }
            println("## $kind p50=${ms(pct(times, .5))} ms min=${ms(times.min())} cpu p50=${ms(pct(cpus, .5))} ms $info")
        }
        st.dispose()
        dir.deleteRecursively()
    }
}
