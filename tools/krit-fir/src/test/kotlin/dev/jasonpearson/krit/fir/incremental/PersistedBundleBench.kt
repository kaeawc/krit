package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.Random

/**
 * What a persisted bundle (#878) saves a restart. Not a regression test: it only runs under the `incrementalBench`
 * Gradle task, by hand, on an otherwise idle machine, and prints a table.
 *
 *  (a) cold start, no bundle: the first check (a full check with the snapshot build) and writing the bundle;
 *  (b) restart with the bundle and [EDITED] edited files: loading it, and the first check, which is served by slices.
 *
 * Properties (Gradle: `-Pkrit.fir.bench.<name>=...`): `files` (synthetic module size, default 2000), `real`
 * (comma-separated source directories checked as they are, copied first; the test JVM's libraries are the classpath),
 * `out` (also append the table to this file).
 */
@Tag(IncrementalPlannerBench.TAG)
class PersistedBundleBench {
    companion object {
        private const val EDITED = 3
        private fun prop(name: String) = System.getProperty("krit.fir.bench.$name")
    }

    @TempDir lateinit var tmp: Path

    private fun say(line: String) { println(line); prop("out")?.let { File(it).appendText(line + "\n") } }

    private fun du(f: File): Long = if (!f.exists()) 0 else f.walkTopDown().filter { it.isFile }.sumOf { it.length() }

    @Test fun restartAgainstColdStart() {
        val real = prop("real")
        if (real != null) {
            val dirs = real.split(',').mapIndexed { i, d -> File(tmp.toFile(), "real-$i-${File(d).name}").also { target -> File(d).copyRecursively(target) } }
            val classpath = System.getProperty("java.class.path").split(File.pathSeparator).filter {
                it.endsWith(".jar") && !File(it).name.startsWith("krit-fir") && !File(it).name.startsWith("krit-rule-api")
            }
            run("real:" + real.split(',').joinToString("+") { File(it).name }, dirs, classpath, null)
        } else {
            val n = (prop("files") ?: "2000").toInt()
            val project = SyntheticProject(tmp.resolve("synthetic-$n").toFile(), n).also { it.generate() }
            run("synthetic:$n", project.sourceDirs.map(::File), listOf(testStdlib), project)
        }
    }

    private fun run(name: String, dirs: List<File>, classpath: List<String>, project: SyntheticProject?) {
        val root = tmp.resolve("fir-incremental").toFile()
        fun files(): List<FileRef> = dirs.flatMap { d -> d.walkTopDown().filter { it.isFile && it.extension == "kt" }.map { it.path } }.sorted()
            .map { FileRef(canon(it), sha1Hex(File(it).readBytes())) }
        fun session() = AnalysisSession(dirs.map { it.path }, classpath, incrementalRecording = true, incrementalPlanning = true)
            .also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); it.configurePersistence(root) }

        val count = files().size
        say("## $name: $count files, ${Runtime.getRuntime().availableProcessors()} cpus, load ${try { File("/proc/loadavg").readText().split(" ").take(3).joinToString("/") } catch (_: Exception) { "?" }}")

        // (a) cold start, no bundle.
        val a = session()
        val t0 = System.nanoTime()
        a.check(0, files(), emptySet())
        val coldMs = (System.nanoTime() - t0) / 1_000_000
        say("(a) cold start, no bundle: first check ${coldMs} ms (plan=${a.lastPlan?.path} ${a.lastPlan?.reason}, snapshot=${a.incrementalState.snapshot != null}, " +
            "load=${(a.persistence?.lastLoad as? BundleLoad.Miss)?.reason})")
        val w0 = System.nanoTime()
        a.flushPersistence()
        val writeMs = (System.nanoTime() - w0) / 1_000_000
        val bundle = root.listFiles()!!.single { it.isDirectory }
        val binSize = File(bundle, BundleStore.BUNDLE_NAME).length()
        val classesSize = bundle.listFiles()!!.filter { it.name.startsWith(BundleStore.CLASSES_PREFIX) }.sumOf { du(it) }
        say("    writing the bundle (after the response, background thread in production): $writeMs ms; bundle.bin ${binSize / 1024} KiB, " +
            "class files ${classesSize / 1024} KiB, total on disk ${du(bundle) / 1024} KiB")
        a.dispose()

        // Edits while nothing was running.
        val rnd = Random(5)
        if (project != null) repeat(EDITED) { project.apply("body") } else {
            val all = dirs.flatMap { d -> d.walkTopDown().filter { it.isFile && it.extension == "kt" }.toList() }
            repeat(EDITED) { all[rnd.nextInt(all.size)].appendText("// edit ${rnd.nextInt()}\n") }
        }

        // (b) restart with the bundle.
        val b = session()
        val t1 = System.nanoTime()
        b.check(1, files(), emptySet())
        val warmMs = (System.nanoTime() - t1) / 1_000_000
        val plan = b.lastPlan!!
        say("(b) restart with bundle, $EDITED edited files: first check $warmMs ms total (load ${b.persistence?.lastLoadMillis} ms: ${b.persistence?.lastLoad?.let { if (it is BundleLoad.Loaded) "Loaded" else "Miss ${(it as BundleLoad.Miss).reason}" }}; " +
            "plan=${plan.path} ${plan.reason ?: ""} changed=${plan.changedFiles} new=${plan.newFiles} dirty=${plan.dirtyFiles} waves=${plan.waves})")
        say("    speedup vs cold start: ${"%.1f".format(coldMs.toDouble() / warmMs)}x")

        // And the next edit on the restarted session, as a steady-state reference.
        if (project != null) project.apply("body") else dirs.flatMap { d -> d.walkTopDown().filter { it.isFile && it.extension == "kt" }.toList() }.let { it[rnd.nextInt(it.size)].appendText("// edit\n") }
        val t2 = System.nanoTime()
        b.check(2, files(), emptySet())
        say("    next edit on the warm session: ${(System.nanoTime() - t2) / 1_000_000} ms (plan=${b.lastPlan?.path})")
        b.dispose()
    }
}
