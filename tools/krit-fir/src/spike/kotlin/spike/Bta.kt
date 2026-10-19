@file:OptIn(org.jetbrains.kotlin.buildtools.api.ExperimentalBuildToolsApi::class)

package spike

import dev.jasonpearson.krit.fir.runner.BundledStdlib
import dev.jasonpearson.krit.fir.runner.resolveJvmTarget
import org.jetbrains.kotlin.buildtools.api.BaseCompilationOperation
import org.jetbrains.kotlin.buildtools.api.CompilationResult
import org.jetbrains.kotlin.buildtools.api.CompilerMessageRenderer
import org.jetbrains.kotlin.buildtools.api.KotlinLogger
import org.jetbrains.kotlin.buildtools.api.KotlinToolchains
import org.jetbrains.kotlin.buildtools.api.SharedApiClassesClassLoader
import org.jetbrains.kotlin.buildtools.api.SourcesChanges
import org.jetbrains.kotlin.buildtools.api.jvm.JvmPlatformToolchain
import org.jetbrains.kotlin.buildtools.api.jvm.operations.JvmCompilationOperation
import java.io.File
import java.net.URLClassLoader

/**
 * F3: Kotlin's own incremental compiler through the Build Tools API (in-process, 2.4.20 impl in its own class loader),
 * with krit's FIR plugin passed as a compiler plugin (the plugin jar relocated like kotlin-compiler-embeddable).
 * Per edit: an IC compile with the known source changes; the files IC recompiles (its "Compile iteration" report) get
 * fresh verdicts from the plugin's diagnostics, every other file keeps its cached verdict. Every --oracle edits the
 * cached state is compared with a non-incremental BTA compile of the whole module (same plugin, same arguments).
 */
class Bta(val o: Map<String, String>) {
    val n = (o["files"] ?: "600").toInt()
    val edits = (o["edits"] ?: "40").toInt()
    val warmup = (o["warmup"] ?: "0").toInt()
    val oracleEvery = (o["oracle"] ?: "1").toInt()
    val firRunner = o["firRunner"] == "true"
    val kinds = o["kinds"]?.split(",")
    val kotlinArgs = o["kotlinArgs"]?.split(",")?.filter { it.isNotBlank() } ?: emptyList()
    val dir = File(SCRATCH, o["dir"] ?: "bta-$n${if (firRunner) "-fir" else ""}")
    val plugin = System.getProperty("spike.embeddablePlugin") ?: error("-Dspike.embeddablePlugin")
    val implCp = File(o["btaCp"] ?: System.getProperty("spike.btaClasspath") ?: error("--btaCp")).readText().trim().split(":")

    class Msg(val severity: CompilerMessageRenderer.Severity, val text: String, val path: String?, val line: Int, val col: Int)

    private val messages = ArrayList<Msg>()
    private val iteration = ArrayList<String>()
    private val infos = ArrayList<String>()
    private var inIteration = false

    private val logger = object : KotlinLogger {
        override val isDebugEnabled = true
        override fun error(msg: String, throwable: Throwable?) { if (o["verbose"] == "true") System.err.println("bta error: $msg ${throwable ?: ""}") }
        override fun warn(msg: String, throwable: Throwable?) {}
        override fun info(msg: String) { infos += msg }
        override fun debug(msg: String) {
            if (msg == "Compile iteration:") { inIteration = true; return }
            if (o["verbose"] == "true") System.err.println("debug: ${msg.take(200)}")
            if (inIteration && msg.startsWith("  ")) iteration += canon(msg.trim().substringBefore(" <- ")) else inIteration = false
        }
        override fun lifecycle(msg: String) {}
    }

    private val renderer = object : CompilerMessageRenderer {
        override fun render(severity: CompilerMessageRenderer.Severity, message: String, location: CompilerMessageRenderer.SourceLocation?): String {
            messages += Msg(severity, message, location?.path?.let(::canon), location?.line ?: 0, location?.column ?: 0)
            if (o["verbose"] == "true" && severity == CompilerMessageRenderer.Severity.ERROR) System.err.println("render ERROR at ${location?.path}:${location?.line} $message")
            return message
        }
    }

    private val ruleRe = Regex("""^\[([A-Za-z][A-Za-z0-9_]*)]""")

    private fun verdicts(files: Collection<String>): Map<String, FileVerdict> {
        val byFile = messages.filter { it.path != null }.groupBy { it.path!! }
        return files.associateWith { f ->
            val ms = byFile[f].orEmpty()
            val findings = ms.filter { it.severity != CompilerMessageRenderer.Severity.INFO && ruleRe.containsMatchIn(it.text) }
                .map { "${ruleRe.find(it.text)!!.groupValues[1]}@${it.line}:${it.col} ${it.text.substringAfter("] ").trim()}" }.sorted()
            val err = ms.any { it.severity == CompilerMessageRenderer.Severity.ERROR && !ruleRe.containsMatchIn(it.text) }
            FileVerdict(findings, err, emptyMap(), ms.filter { it.severity != CompilerMessageRenderer.Severity.INFO && it.severity != CompilerMessageRenderer.Severity.DEBUG }
                .map { "${it.severity.name[0]}|${it.line}:${it.col}|${it.text}" }.sorted())
        }
    }

    fun run() {
        dir.deleteRecursively(); dir.mkdirs()
        val g = makeProj(o, File(dir, "src"), n, 7L)
        println("# ${g.describe()} bta firRunner=$firRunner load=${load1()}")
        val loader = URLClassLoader(implCp.map { File(it).toURI().toURL() }.toTypedArray(), SharedApiClassesClassLoader())
        val tc = KotlinToolchains.loadImplementation(loader)
        println("# BTA compiler version ${tc.getCompilerVersion()}")
        val jvm = tc.getToolchain(JvmPlatformToolchain::class.java)
        val session = tc.createBuildSession()
        val cp = BundledStdlib.effectiveClasspath(g.classpath)
        var t0 = System.nanoTime()
        val snapFiles = cp.mapIndexed { i, entry ->
            val snap = session.executeOperation(jvm.classpathSnapshottingOperationBuilder(File(entry).toPath()).build(), tc.createInProcessExecutionPolicy(), logger)
            File(dir, "cp-snap-$i.bin").also { snap.saveSnapshot(it) }.toPath()
        }
        println("classpath snapshots: ${cp.size} entries ${ms(System.nanoTime() - t0)} ms")
        val out = File(dir, "classes"); val work = File(dir, "ic")

        fun args(b: JvmCompilationOperation.Builder) {
            val a = mutableListOf(
                "-Xplugin=$plugin", "-no-stdlib", "-no-reflect", "-classpath", cp.joinToString(File.pathSeparator),
                "-module-name", "main", "-jvm-target", resolveJvmTarget(""), "-Xreport-all-warnings",
            )
            a += kotlinArgs
            b.compilerArguments.applyArgumentStrings(a)
            b[BaseCompilationOperation.COMPILER_MESSAGE_RENDERER] = renderer
        }

        fun compile(changes: SourcesChanges?, dest: File, ic: Boolean): Pair<CompilationResult, Long> {
            messages.clear(); iteration.clear(); infos.clear(); inIteration = false
            val sources = g.texts.keys.map { File(it).toPath() }
            val b = jvm.jvmCompilationOperationBuilder(sources, dest.toPath())
            args(b)
            if (ic) {
                val icb = b.snapshotBasedIcConfigurationBuilder(work.toPath(), changes ?: SourcesChanges.Unknown, snapFiles)
                // USE_FIR_RUNNER (IncrementalFirJvmCompilerRunner) is deprecated at ERROR level in 2.4.20 ("removed in 2.5.0")
                b[JvmCompilationOperation.INCREMENTAL_COMPILATION] = icb.build()
            }
            val t = System.nanoTime()
            val r = session.executeOperation(b.build(), tc.createInProcessExecutionPolicy(), logger)
            return r to (System.nanoTime() - t)
        }

        // cold IC build
        t0 = System.nanoTime(); val c0 = cpuNanos()
        val (r0, d0) = compile(SourcesChanges.Unknown, out, ic = true)
        val all0 = g.texts.keys.map(::canon)
        val cache = HashMap(verdicts(all0))
        println("cold IC build: ${ms(d0)} ms cpu=${ms(cpuNanos() - c0)} result=$r0 recompiled=${iteration.size} findings=${cache.values.sumOf { it.findings.size }} errFiles=${cache.values.count { it.error }} icCaches=${du(work)}KB classes=${du(out)}KB")
        var rebuilds = 0; var staleEdits = 0; var staleFiles = 0; var strictEdits = 0; var compared = 0; var missedByIc = 0
        val rows = ArrayList<Triple<String, Long, Int>>()
        var prevRef: Map<String, FileVerdict>? = null
        for (i in 1..(warmup + edits)) {
            val e = g.nextEdit(kinds)
            val modified = (e.changed + e.added).map { File(it) }
            val removed = e.deleted.map { File(it) }
            val c = cpuNanos()
            val (r, d) = compile(SourcesChanges.Known(modified, removed), out, ic = true)
            val rebuild = infos.any { it.contains("Non-incremental compilation will be performed") }
            if (rebuild) rebuilds++
            for (f in removed) cache.remove(canon(f.path))
            val recompiled = iteration.toSet()
            cache.putAll(verdicts(recompiled))
            var line = "e$i ${e.kind} ${ms(d)} ms cpu=${ms(cpuNanos() - c)} result=$r recompiled=${recompiled.size}${if (rebuild) " REBUILD(" + infos.first { it.contains("Non-incremental") }.take(120) + ")" else ""} icCaches=${du(work)}KB load=${load1()}"
            if (i > warmup) rows += Triple(e.kind, d, recompiled.size)
            if (oracleEvery > 0 && i % oracleEvery == 0) {
                val refDir = File(dir, "ref").also { it.deleteRecursively(); it.mkdirs() }
                compile(null, refDir, ic = false)
                val all = g.texts.keys.map(::canon)
                val ref = verdicts(all)
                val vd = Compare.verdictDiff(ref, cache, all); val sd = Compare.strictDiff(ref, cache, all)
                compared++
                if (vd.isNotEmpty()) { staleEdits++; staleFiles += vd.size }
                if (sd.isNotEmpty()) strictEdits++
                line += " | ref: verdictStale=${vd.size} strictStale=${sd.size}"
                vd.take(3).forEach { line += "\n     STALE " + Compare.explain(it, ref[it], cache[it]) }
                val pr = prevRef
                if (pr != null) {
                    val truth = all.filter { it !in modified.map { m -> canon(m.path) } && pr[it] != ref[it] }.toSet()
                    val missed = truth - recompiled
                    missedByIc += missed.size
                    line += " | truth=${truth.size} missedByIc=${missed.size}"
                    if (missed.isNotEmpty()) line += "\n     IC did not recompile: ${missed.take(4).map { File(it).name }}"
                }
                prevRef = ref
                if (o["resync"] == "true") cache.putAll(ref)
            }
            println(line)
        }
        val ds = rows.map { it.second }
        println("## bta firRunner=$firRunner n=${rows.size} p50=${ms(pct(ds, .5))} p95=${ms(pct(ds, .95))} ms rebuilds=$rebuilds icCaches=${du(work)}KB classes=${du(out)}KB")
        for (k in rows.map { it.first }.distinct().sorted()) {
            val x = rows.filter { it.first == k }
            println("  %-9s n=%3d p50=%6d ms recompiled p50=%d max=%d".format(k, x.size, ms(pct(x.map { it.second }, .5)), pct(x.map { it.third.toLong() }, .5), x.maxOf { it.third }))
        }
        if (compared > 0) println("## correctness: compared=$compared verdictStale edits=$staleEdits files=$staleFiles strictStale edits=$strictEdits truth-not-recompiled=$missedByIc")
        println("## memory at end: ${memSnap(true)}")
        session.close()
        if (o["keepDir"] != "true") dir.deleteRecursively()
    }

    private fun du(f: File): Long = f.walkTopDown().filter { it.isFile }.sumOf { it.length() } / 1024
}
