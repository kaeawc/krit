package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * Go passes `kotlin`/`java` directories below the scan roots as sourceDirs, but requests every `.kt` the scan found
 * (`tools/Extra.kt`, a single-file scan path, ...). Those files belong to the repository, so a bundle that names them
 * loads; a bundle that names files outside the repository (and outside the source directories) does not.
 */
class PersistedBundleRepoRootTest {
    @TempDir lateinit var tmp: Path

    private val repo get() = tmp.resolve("repo").toFile()
    private val root get() = File(repo, ".krit/fir-incremental")
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()

    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun write(rel: String, text: String, under: File = repo) = under.writeSource(rel, text)

    private fun session(sourceDirs: List<File>): AnalysisSession =
        AnalysisSession(sourceDirs.map { it.path }, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true).also {
            it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); it.configurePersistence(root); sessions += it
        }

    private fun persistThenRestart(sourceDirs: List<File>, files: List<String>, edit: () -> Unit = {}): AnalysisSession {
        val refs = files.map { FileRef(it) }
        val first = session(sourceDirs)
        first.check(1, refs, rules)
        first.flushPersistence(); first.dispose(); sessions.remove(first)
        edit()
        return session(sourceDirs).also { it.check(2, refs, rules) }
    }

    @Test fun aRequestedFileOutsideTheSourceDirsButInsideTheRepoLoads() {
        val kotlin = File(repo, "src/main/kotlin")
        write("A.kt", "package p\n\nfun protocolProbe() {}\nfun a(): Int = 1\n", kotlin)
        write("tools/Extra.kt", "package p\n\nfun extra() { protocolProbe() }\n")
        val files = listOf(File(kotlin, "A.kt").path, File(repo, "tools/Extra.kt").path)
        val restarted = persistThenRestart(listOf(kotlin), files) { write("tools/Extra.kt", "package p\n\nfun extra() { protocolProbe(); a() }\n") }
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "${restarted.persistence!!.lastLoad}")
        assertEquals(PlanPath.INCREMENTAL, assertNotNull(restarted.lastPlan).path)
    }

    @Test fun aSingleFileScanPathLoads() {
        val kotlin = File(repo, "src/main/kotlin")
        write("B.kt", "package p\n\nfun protocolProbe() {}\nfun b(): Int = 1\n", kotlin)
        // The scan root is one file: nothing named kotlin/ java is below it, so there are no source directories.
        val restarted = persistThenRestart(emptyList(), listOf(File(kotlin, "B.kt").path))
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "${restarted.persistence!!.lastLoad}")
    }

    @Test fun aSessionWithoutSourceDirsLoadsForFilesInsideTheRepo() {
        write("src/A.kt", "package p\n\nfun protocolProbe() {}\n")
        write("src/B.kt", "package p\n\nfun b() { protocolProbe() }\n")
        val files = listOf(File(repo, "src/A.kt").path, File(repo, "src/B.kt").path)
        val restarted = persistThenRestart(emptyList(), files) { write("src/B.kt", "package p\n\nfun b() { protocolProbe(); protocolProbe() }\n") }
        assertTrue(restarted.persistence!!.lastLoad is BundleLoad.Loaded, "${restarted.persistence!!.lastLoad}")
        assertEquals(PlanPath.INCREMENTAL, assertNotNull(restarted.lastPlan).path)
    }

    @Test fun aFileOutsideTheRepoAndTheSourceDirsStillRejectsTheBundle() {
        val kotlin = File(repo, "src/main/kotlin")
        write("A.kt", "package p\n\nfun protocolProbe() {}\n", kotlin)
        val stray = File(tmp.toFile(), "elsewhere").also { write("Stray.kt", "package p\n\nfun stray() { protocolProbe() }\n", it) }
        val files = listOf(File(kotlin, "A.kt").path, File(stray, "Stray.kt").path)
        val restarted = persistThenRestart(listOf(kotlin), files)
        val load = restarted.persistence!!.lastLoad
        assertTrue(load is BundleLoad.Miss && load.reason == BundleRejected.Reason.STATE_MISMATCH, "$load")
    }
}
