package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.runner.AnalysisSession
import dev.jasonpearson.krit.fir.runner.BundleLoad
import dev.jasonpearson.krit.fir.runner.FileRef
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** The roots a bundle may name: the repository is where `.krit/fir-incremental` is, however it is reached. */
class PersistedBundleRootsTest {
    @TempDir lateinit var tmp: Path
    private val rules = setOf("ProtocolProbe")
    private val sessions = ArrayList<AnalysisSession>()
    @AfterEach fun cleanUp() { sessions.forEach { it.dispose() } }

    private fun session(root: File?, sourceDirs: List<String>): AnalysisSession =
        AnalysisSession(sourceDirs, listOf(testStdlib), incrementalRecording = true, incrementalPlanning = true).also {
            it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); it.configurePersistence(root); sessions += it
        }

    // A `.krit` that links elsewhere would move the repository to the link target's parent: the root is refused instead.
    @Test fun aSymlinkedDotKritIsRefused() {
        val repo = tmp.resolve("repo").toFile().also { it.mkdirs() }
        val elsewhere = tmp.resolve("elsewhere").toFile().also { it.mkdirs() }
        File(elsewhere, ".krit").mkdirs()
        Files.createSymbolicLink(File(repo, ".krit").toPath(), File(elsewhere, ".krit").toPath())
        val warnings = ArrayList<String>()
        assertNull(BundleStore.trustedRoot(File(repo, ".krit/fir-incremental").path, warn = { warnings += it }))
        assertEquals(1, warnings.size)
        // A `fir-incremental` that is a link is refused too.
        val repo2 = tmp.resolve("repo2").toFile().also { File(it, ".krit").mkdirs() }
        Files.createSymbolicLink(File(repo2, ".krit/fir-incremental").toPath(), elsewhere.toPath())
        assertNull(BundleStore.trustedRoot(File(repo2, ".krit/fir-incremental").path, warn = {}))
    }

    @Test fun aRootWithoutTheDotKritLayoutTrustsOnlyTheSourceDirs() {
        val odd = tmp.resolve("odd-root").toFile()
        val kotlin = tmp.resolve("proj/kotlin").toFile()
        kotlin.writeSource("A.kt", "package p\n\nfun protocolProbe() {}\n")
        tmp.resolve("outside").toFile().writeSource("X.kt", "package p\n\nfun x() { protocolProbe() }\n")
        val files = listOf(File(kotlin, "A.kt").path, tmp.resolve("outside/X.kt").toString()).map { FileRef(it) }
        val first = session(odd, listOf(kotlin.path)); first.check(1, files, rules); first.flushPersistence(); first.dispose(); sessions.remove(first)
        val second = session(odd, listOf(kotlin.path)); second.check(2, files, rules)
        val load = second.persistence!!.lastLoad
        assertTrue(load is BundleLoad.Miss && load.reason == BundleRejected.Reason.STATE_MISMATCH, "no repository was derived: $load")
    }

    @Test fun anyDirectoryNamedDotKritIsATrustedRootAndItsParentTheRepo() {
        val home = tmp.resolve("home").toFile().also { File(it, ".krit").mkdirs() }
        val trusted = assertNotNull(BundleStore.trustedRoot(File(home, ".krit/fir-incremental").path, warn = {}))
        assertEquals(home.canonicalFile, trusted.parentFile.parentFile)
    }

    // The request spells the repository through a symlink (macOS /var and /tmp, a linked ~/code): Go sends absolute
    // but not canonical paths, findings keep that spelling, and the roots are canonical.
    @Test fun aRepositoryReachedThroughASymlinkStillLoads() {
        val real = tmp.resolve("real").toFile().also { it.mkdirs() }
        val link = tmp.resolve("link").toFile()
        Files.createSymbolicLink(link.toPath(), real.toPath())
        val repo = File(link, "repo")
        val kotlin = File(repo, "src/main/kotlin")
        kotlin.writeSource("A.kt", "package p\n\nfun protocolProbe() {}\n")
        kotlin.writeSource("B.kt", "package p\n\nfun b() { protocolProbe() }\n")
        val root = BundleStore.trustedRoot(File(repo, ".krit/fir-incremental").path, warn = {})!! // canonical, as in Main
        val files = listOf(File(kotlin, "A.kt").path, File(kotlin, "B.kt").path).map { FileRef(it) }
        val first = session(root, listOf(kotlin.path)); first.check(1, files, rules); first.flushPersistence(); first.dispose(); sessions.remove(first)
        val second = session(root, listOf(kotlin.path)); second.check(2, files, rules)
        println("PROBE symlinked spelling: " + second.persistence!!.lastLoad.let { if (it is BundleLoad.Miss) "Miss " + it.reason + " " + it.detail else it.toString() })
        assertTrue(second.persistence!!.lastLoad is BundleLoad.Loaded, "${second.persistence!!.lastLoad}")
    }
}
