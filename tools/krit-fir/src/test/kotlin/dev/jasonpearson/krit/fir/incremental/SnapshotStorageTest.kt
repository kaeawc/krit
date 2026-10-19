package dev.jasonpearson.krit.fir.incremental

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * [SnapshotStorage] deletes directories recursively, so what it may delete is pinned here: only snapshot
 * directories nobody owns, decided by an OS file lock and not by a process id.
 */
class SnapshotStorageTest {
    @TempDir lateinit var tmp: Path

    private val parent get() = tmp.resolve("cache").toFile()
    private val snapshots get() = File(parent, "fir-snapshots")

    private fun dir(name: String, withLock: Boolean = true): File =
        File(snapshots, name).also {
            it.mkdirs()
            File(it, "classes").mkdirs()
            File(it, "classes/A.class").writeText("x")
            if (withLock) File(it, ".lock").writeText("")
        }

    /** A separate JVM that holds the lock of [lockFile] until its stdin closes. */
    private fun holdLockInAnotherProcess(lockFile: File): Process {
        val source = tmp.resolve("Hold.java").toFile()
        source.writeText(
            """
            import java.nio.channels.FileChannel;
            import java.nio.file.Path;
            import java.nio.file.StandardOpenOption;
            public class Hold {
                public static void main(String[] args) throws Exception {
                    FileChannel channel = FileChannel.open(Path.of(args[0]), StandardOpenOption.CREATE, StandardOpenOption.WRITE);
                    channel.lock();
                    System.out.println("locked");
                    System.out.flush();
                    System.in.read();
                }
            }
            """.trimIndent(),
        )
        val java = File(System.getProperty("java.home"), "bin/java").path
        val process = ProcessBuilder(java, source.path, lockFile.path).redirectError(ProcessBuilder.Redirect.DISCARD).start()
        val line = BufferedReader(InputStreamReader(process.inputStream)).readLine()
        assertEquals("locked", line)
        return process
    }

    // ---- cacheDir precedence ----------------------------------------------------------------------------------

    private fun dirFor(
        property: String? = null, env: String? = null, os: String = "Linux", home: String = "/home/u",
        xdg: String? = null, localAppData: String? = null,
    ) = SnapshotStorage.cacheDirFor(property, env, os, home, xdg, localAppData, "/tmp", "u").path

    @Test fun theSystemPropertyBeatsTheEnvironmentWhichBeatsTheOsDefault() {
        assertEquals("/prop", dirFor(property = "/prop", env = "/env", xdg = "/xdg"))
        assertEquals("/env", dirFor(property = "", env = "/env", xdg = "/xdg"))
        assertEquals("/env", dirFor(env = "/env"))
        assertEquals("/xdg/krit", dirFor(xdg = "/xdg"))
    }

    @Test fun theOsDefaultFollowsTheGoSidesUserCacheDir() {
        assertEquals("/home/u/.cache/krit", dirFor())
        assertEquals("/home/u/Library/Caches/krit", dirFor(os = "Mac OS X"))
        assertEquals("C:\\Local/krit", dirFor(os = "Windows 11", localAppData = "C:\\Local"))
        assertEquals("/tmp/krit-u", dirFor(home = ""))
        assertEquals("/tmp/krit-u", dirFor(os = "Windows 11"))
    }

    // ---- the sweep --------------------------------------------------------------------------------------------

    @Test fun theSweepRemovesAnUnownedDirectoryAndKeepsThisProcessesOwn() {
        val dead = dir("p1-1-dead")
        val own = SnapshotStorage.newSnapshotRoot(parent)
        // newSnapshotRoot swept the dead one already.
        assertFalse(dead.exists())
        val deadAgain = dir("p1-2-dead")
        assertEquals(1, SnapshotStorage.sweepDead(snapshots))
        assertFalse(deadAgain.exists())
        assertTrue(own.isDirectory, "this process's own snapshot stays")
        assertTrue(File(own, ".lock").isFile)
        SnapshotStorage.release(own)
        own.deleteRecursively()
    }

    @Test fun theSweepKeepsADirectoryAnotherProcessHoldsAndRemovesItOnceItLetsGo() {
        val live = dir("p4242-1-live")
        val dead = dir("p4243-1-dead")
        val holder = holdLockInAnotherProcess(File(live, ".lock"))
        try {
            // Whatever pid the name carries: the lock decides.
            assertEquals(1, SnapshotStorage.sweepDead(snapshots))
            assertTrue(live.isDirectory, "a live owner's snapshot is kept")
            assertFalse(dead.exists())
        } finally {
            holder.outputStream.close()
            holder.waitFor()
        }
        assertEquals(1, SnapshotStorage.sweepDead(snapshots))
        assertFalse(live.exists(), "after its owner is gone the directory is swept")
    }

    @Test fun aDirectoryWithoutALockFileIsKeptUntilItIsOld() {
        val fresh = dir("p9-1-fresh", withLock = false)
        val old = dir("p9-2-old", withLock = false)
        old.setLastModified(System.currentTimeMillis() - SnapshotStorage.LOCKLESS_GRACE_MILLIS - 60_000)
        assertEquals(1, SnapshotStorage.sweepDead(snapshots))
        assertTrue(fresh.isDirectory)
        assertFalse(old.exists())
    }

    @Test fun theSweepIgnoresPlainFilesAndTheOwnersLockedDirectoriesAreNeverOverlapped() {
        File(snapshots, "stray.txt").also { it.parentFile.mkdirs(); it.writeText("x") }
        val own = SnapshotStorage.newSnapshotRoot(parent)
        assertEquals(0, SnapshotStorage.sweepDead(snapshots))
        assertTrue(File(snapshots, "stray.txt").isFile)
        assertTrue(own.isDirectory)
        SnapshotStorage.release(own)
        own.deleteRecursively()
    }

    /** Whether a separate JVM can take the lock of [lockFile] (true: nobody in any process holds it). */
    private fun lockIsFreeForAnotherProcess(lockFile: File): Boolean {
        val source = tmp.resolve("Probe.java").toFile()
        source.writeText(
            """
            import java.nio.channels.FileChannel;
            import java.nio.file.Path;
            import java.nio.file.StandardOpenOption;
            public class Probe {
                public static void main(String[] args) throws Exception {
                    FileChannel channel = FileChannel.open(Path.of(args[0]), StandardOpenOption.WRITE);
                    System.out.println(channel.tryLock() != null ? "free" : "held");
                }
            }
            """.trimIndent(),
        )
        val java = File(System.getProperty("java.home"), "bin/java").path
        val process = ProcessBuilder(java, source.path, lockFile.path).redirectError(ProcessBuilder.Redirect.DISCARD).start()
        val line = BufferedReader(InputStreamReader(process.inputStream)).readLine()
        process.waitFor()
        return line == "free"
    }

    @Test fun sweepingThroughAnotherSpellingOfThePathKeepsThisProcessesLock() {
        val own = SnapshotStorage.newSnapshotRoot(parent)
        val lockFile = File(own, ".lock")
        assertFalse(lockIsFreeForAnotherProcess(lockFile), "the lock is held to begin with")
        // A symlink to the cache root, and a path with a detour: the same directories under other names.
        val link = tmp.resolve("link-to-cache").also { java.nio.file.Files.createSymbolicLink(it, parent.toPath()) }
        SnapshotStorage.sweepDead(File(link.toFile(), "fir-snapshots"))
        SnapshotStorage.sweepDead(File(parent, "x/../fir-snapshots"))
        assertTrue(own.isDirectory)
        assertFalse(lockIsFreeForAnotherProcess(lockFile), "sweeping must not release this process's own lock")
        SnapshotStorage.release(own)
        assertTrue(lockIsFreeForAnotherProcess(lockFile), "release does")
        own.deleteRecursively()
    }

    @Test fun creatingSnapshotsWhileOthersSweepNeverLosesADirectory() {
        val roots = java.util.concurrent.ConcurrentLinkedQueue<File>()
        val stop = java.util.concurrent.atomic.AtomicBoolean(false)
        val sweeper = Thread { while (!stop.get()) SnapshotStorage.sweepDead(snapshots) }
        sweeper.start()
        try {
            val creators = (1..4).map {
                Thread { repeat(25) { roots += SnapshotStorage.newSnapshotRoot(parent) } }.also { t -> t.start() }
            }
            creators.forEach { it.join() }
        } finally {
            stop.set(true)
            sweeper.join()
        }
        assertEquals(100, roots.size)
        assertTrue(roots.all { it.isDirectory && File(it, ".lock").isFile }, "a concurrent sweep deleted a live snapshot")
        roots.forEach { SnapshotStorage.release(it); it.deleteRecursively() }
    }

    // ---- the shutdown hook ------------------------------------------------------------------------------------

    @Test fun theShutdownHookRemovesOnlyThisProcessesDirectories() {
        val a = SnapshotStorage.newSnapshotRoot(parent)
        val b = SnapshotStorage.newSnapshotRoot(parent)
        val foreign = dir("p77-1-foreign")
        val holder = holdLockInAnotherProcess(File(foreign, ".lock"))
        try {
            SnapshotStorage.removeOwned()
            assertFalse(a.exists())
            assertFalse(b.exists())
            assertTrue(foreign.isDirectory, "a directory this process does not own is not touched")
        } finally {
            holder.outputStream.close()
            holder.waitFor()
        }
    }

    @Test fun deletingASnapshotReleasesItsLockAndItsDirectory() {
        val root = SnapshotStorage.newSnapshotRoot(parent)
        val snapshot = Snapshot(root)
        snapshot.delete()
        assertFalse(root.exists())
        // Nothing of this process is held any more.
        SnapshotStorage.removeOwned()
        assertEquals(0, SnapshotStorage.sweepDead(snapshots))
    }
}
