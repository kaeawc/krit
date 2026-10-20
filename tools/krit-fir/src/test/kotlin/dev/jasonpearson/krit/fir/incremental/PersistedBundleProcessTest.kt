package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BatchResult
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.concurrent.TimeUnit
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A bundle written by one JVM starts a different JVM warm (#878): the first check after a restart, with files edited
 * while nothing was running, is served incrementally and equals a fresh full check.
 */
class PersistedBundleProcessTest {
    @TempDir lateinit var tmp: Path

    private val src get() = tmp.resolve("src").toFile()
    private val root get() = tmp.resolve("fir-incremental").toFile()

    private fun write(rel: String, text: String) = src.writeSource(rel, text)

    private fun run(): List<String> {
        val java = File(System.getProperty("java.home"), "bin/java").path
        val command = listOf(
            java, "-Xmx1g",
            "-Dkrit.fir.plugin.jar=${System.getProperty("krit.fir.plugin.jar")}",
            "-Dkrit.fir.cache.dir=${System.getProperty("krit.fir.cache.dir") ?: tmp.resolve("cache").toString()}",
            "-cp", System.getProperty("java.class.path"),
            "dev.jasonpearson.krit.fir.incremental.PersistedBundleProcess", src.path, root.path, tmp.resolve("snapshots").toString(),
        )
        val process = ProcessBuilder(command).redirectError(ProcessBuilder.Redirect.DISCARD).start()
        val lines = process.inputStream.bufferedReader().readLines()
        assertTrue(process.waitFor(300, TimeUnit.SECONDS), "the child JVM finished")
        assertEquals(0, process.exitValue(), "child output: $lines")
        return lines
    }

    @Test fun aSecondProcessStartsWarmFromTheFirstProcessesBundle() {
        write("Api.kt", "package p\n\nfun protocolProbe() {}\nfun base(): Int = 1\n")
        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base() }\n")
        write("Other.kt", "package p\n\nfun o() { protocolProbe() }\n")

        val cold = run()
        assertTrue("plan=FULL reason=COLD load=Miss:MISSING" in cold, "$cold")

        write("Use.kt", "package p\n\nfun u() { protocolProbe(); base(); base() }\n")
        write("Fresh.kt", "package p\n\nfun fresh() { protocolProbe() }\n")
        val warm = run()
        assertTrue("plan=INCREMENTAL reason=null load=Loaded changed=1 new=1 deleted=0" in warm, "$warm")

        val want = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = false, incrementalPlanning = false)
            .check(1, kotlinFilesIn(src).map { FileRef(it) }, setOf("ProtocolProbe"))
        assertEquals(describe(want), warm.filter { it.startsWith("finding ") }.sorted())

        // The process that served incrementally also left a bundle the next one can use.
        File(src, "Other.kt").delete()
        val again = run()
        assertTrue("plan=INCREMENTAL reason=null load=Loaded changed=0 new=0 deleted=1" in again, "$again")
    }

    private fun describe(r: BatchResult): List<String> =
        r.findings.map { "finding ${it.path} ${it.rule}@${it.line}:${it.col}" }.sorted()
}

/** What the child JVM of [PersistedBundleProcessTest] runs: one planned check of the module, then a clean shutdown. */
object PersistedBundleProcess {
    @JvmStatic fun main(args: Array<String>) {
        val (srcDir, root, snapshots) = args.toList()
        val src = File(srcDir)
        val session = AnalysisSession(listOf(src.path), listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true)
        session.snapshotRootOverride = File(snapshots)
        session.configurePersistence(File(root))
        val result = session.check(1, kotlinFilesIn(src).map { FileRef(it) }, setOf("ProtocolProbe"))
        val plan = session.lastPlan!!
        val load = when (val l = session.persistence?.lastLoad) {
            null -> "none"
            is BundleLoad.Loaded -> "Loaded"
            is BundleLoad.Miss -> "Miss:${l.reason}"
        }
        println(
            "plan=${plan.path} reason=${plan.reason} load=$load" +
                if (plan.path == PlanPath.INCREMENTAL) " changed=${plan.changedFiles} new=${plan.newFiles} deleted=${plan.deletedFiles}" else "",
        )
        for (f in result.findings) println("finding ${f.path} ${f.rule}@${f.line}:${f.col}")
        session.dispose() // flushes the bundle
        System.out.flush()
        kotlin.system.exitProcess(0)
    }
}
