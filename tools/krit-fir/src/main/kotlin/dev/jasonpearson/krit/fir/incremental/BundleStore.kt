package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.io.IOException
import java.nio.channels.FileChannel
import java.nio.channels.FileLock
import java.nio.channels.OverlappingFileLockException
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.FileVisitResult
import java.nio.file.Files
import java.nio.file.NoSuchFileException
import java.nio.file.Path
import java.nio.file.SimpleFileVisitor
import java.nio.file.StandardCopyOption
import java.nio.file.StandardOpenOption
import java.nio.file.attribute.BasicFileAttributes
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import java.util.concurrent.locks.ReentrantLock

/**
 * The on-disk home of persisted incremental bundles, a directory the Go side supplies (`{repo}/.krit/fir-incremental`).
 *
 * One bundle per [compileKey][IncrementalState.compileKey] and format version, in `v<version>-<compileKey>/`:
 *  - `bundle.bin`: the state and the verdict cache, one file (see [BundleCodec]);
 *  - `classes-<id>/`: the snapshot's class files, named by the id `bundle.bin` records.
 *
 * Publishing is rename-based. The class files are copied into a staging directory and renamed to `classes-<id>`, which
 * no bundle references yet; then `bundle.bin` is written to a temp file and renamed over the old one. A reader takes no
 * lock: it reads `bundle.bin` whole, so it sees the old bundle or the new one, and the class directory a bundle names
 * is never modified once it has its name. The two newest class directories are kept, so a reader that picked up the
 * previous `bundle.bin` just before a publish still finds its classes; if it does not, that is a miss (a cold check),
 * never an error.
 *
 * Writers serialise on a lock file next to the bundle directory: an OS file lock across processes, and a mutex per path
 * inside this JVM (on Linux closing any channel on a file drops the process's lock on it, so one channel at a time).
 * Only the [keep] most recently used bundles are kept; the others are deleted by whoever publishes next, if nobody is
 * writing them.
 */
internal class BundleStore(val root: File, private val keep: Int = DEFAULT_KEEP) {
    fun dir(compileKey: String): File = File(root, "v${BundleCodec.VERSION}-$compileKey")

    fun bundleFile(compileKey: String): File = File(dir(compileKey), BUNDLE_NAME)

    fun classesDir(compileKey: String, id: String): File = File(dir(compileKey), "$CLASSES_PREFIX$id")

    private fun lockFile(compileKey: String): File = File(root, "v${BundleCodec.VERSION}-$compileKey.lock")

    /** Reads and checks the bundle of [compileKey]; throws [BundleRejected] when there is none or it cannot be used. */
    fun read(compileKey: String): DecodedBundle {
        val file = bundleFile(compileKey)
        val bytes = try {
            Files.readAllBytes(file.toPath())
        } catch (_: NoSuchFileException) {
            throw BundleRejected(BundleRejected.Reason.MISSING, "no bundle for this compile key")
        } catch (e: IOException) {
            throw BundleRejected(BundleRejected.Reason.IO, "${e.javaClass.simpleName}: ${e.message}")
        }
        val decoded = BundleCodec.decode(bytes, compileKey)
        // Recency for eviction is use, not just writes.
        try { file.setLastModified(System.currentTimeMillis()) } catch (_: Exception) {}
        return decoded
    }

    /** A fresh empty directory next to where `classes-<id>` will go, for the caller to fill and hand to [publish]. */
    fun newStage(compileKey: String): File {
        val dir = dir(compileKey)
        Files.createDirectories(dir.toPath())
        return File(dir, "$STAGE_PREFIX${UUID.randomUUID()}").also { Files.createDirectory(it.toPath()) }
    }

    /**
     * Publishes [bytes] as the bundle of [compileKey]. [staged] is a directory from [newStage] holding the snapshot
     * class files, which becomes `classes-<snapshotId>`; null when that directory was published already, in which case
     * nothing is published if it has gone (another process replaced it), since a bundle must not name classes that are
     * not there. Returns false when it was not published: the lock was not available in time, or the classes vanished.
     */
    fun publish(compileKey: String, bytes: ByteArray, snapshotId: String, staged: File?): Boolean {
        Files.createDirectories(root.toPath())
        val result = withLock(lockFile(compileKey), LOCK_WAIT_MILLIS) {
            val dir = dir(compileKey)
            Files.createDirectories(dir.toPath())
            val target = classesDir(compileKey, snapshotId)
            if (staged != null) {
                if (target.exists()) target.deleteRecursively()
                moveAtomically(staged.toPath(), target.toPath())
            } else if (!target.isDirectory) {
                return@withLock false
            }
            val tmp = File(dir, "$TEMP_PREFIX${UUID.randomUUID()}")
            try {
                FileChannel.open(tmp.toPath(), StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE).use { ch ->
                    val buf = java.nio.ByteBuffer.wrap(bytes)
                    while (buf.hasRemaining()) ch.write(buf)
                    ch.force(true)
                }
                moveAtomically(tmp.toPath(), bundleFile(compileKey).toPath())
            } finally {
                tmp.delete()
            }
            pruneClasses(dir, keepNewest = 2, pinned = target)
            sweepLeftovers(dir)
            evict(except = compileKey)
            true
        }
        if (result != true && staged != null) staged.deleteRecursively()
        return result == true
    }

    /** Deletes the bundles beyond the [keep] most recently used, never [except] and never one that is being written. */
    fun evict(except: String? = null) {
        val prefix = "v${BundleCodec.VERSION}-"
        val dirs = root.listFiles { f -> f.isDirectory && f.name.startsWith("v") }.orEmpty()
        // Older format versions are bundles too: a version bump must not leave the old ones behind forever.
        val ranked = dirs.sortedByDescending { recency(it) }
        val protectedName = except?.let { prefix + it }
        var kept = 0
        for (dir in ranked) {
            if (dir.name == protectedName) { kept++; continue }
            if (kept < keep) { kept++; continue }
            val lock = File(root, dir.name + ".lock")
            withLock(lock, 0) {
                dir.deleteRecursively()
                lock.delete()
            }
        }
    }

    private fun recency(dir: File): Long {
        val bundle = File(dir, BUNDLE_NAME)
        return if (bundle.isFile) bundle.lastModified() else dir.lastModified()
    }

    private fun pruneClasses(dir: File, keepNewest: Int, pinned: File) {
        val all = dir.listFiles { f -> f.isDirectory && f.name.startsWith(CLASSES_PREFIX) }.orEmpty()
        val doomed = all.filter { it != pinned }.sortedByDescending { it.lastModified() }.drop(keepNewest - 1)
        for (old in doomed) old.deleteRecursively()
    }

    // Staging directories and temp files of writers that died; one that is young may be another process's.
    private fun sweepLeftovers(dir: File) {
        val cutoff = System.currentTimeMillis() - LEFTOVER_MILLIS
        for (f in dir.listFiles().orEmpty()) {
            if ((f.name.startsWith(STAGE_PREFIX) || f.name.startsWith(TEMP_PREFIX)) && f.lastModified() < cutoff) f.deleteRecursively()
        }
    }

    private fun moveAtomically(from: Path, to: Path) {
        try {
            Files.move(from, to, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
        } catch (_: AtomicMoveNotSupportedException) {
            Files.move(from, to, StandardCopyOption.REPLACE_EXISTING)
        }
    }

    companion object {
        const val DEFAULT_KEEP = 3
        const val BUNDLE_NAME = "bundle.bin"
        const val CLASSES_PREFIX = "classes-"
        private const val STAGE_PREFIX = ".stage-"
        private const val TEMP_PREFIX = ".bundle-"
        private const val LOCK_WAIT_MILLIS = 5_000L
        private const val LEFTOVER_MILLIS = 60 * 60 * 1000L

        // One mutex per lock file in this JVM: see the class comment.
        private val mutexes = ConcurrentHashMap<String, ReentrantLock>()

        /** Runs [body] holding the lock on [lockFile], waiting up to [waitMillis]; null if the lock was not obtained. */
        fun <T> withLock(lockFile: File, waitMillis: Long, body: () -> T): T? {
            val mutex = mutexes.computeIfAbsent(try { lockFile.canonicalPath } catch (_: IOException) { lockFile.absolutePath }) { ReentrantLock() }
            if (!mutex.tryLock(waitMillis, TimeUnit.MILLISECONDS)) return null
            try {
                val channel = try {
                    FileChannel.open(lockFile.toPath(), StandardOpenOption.CREATE, StandardOpenOption.WRITE)
                } catch (_: IOException) { return null }
                channel.use {
                    val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(waitMillis)
                    var lock: FileLock?
                    while (true) {
                        lock = try { channel.tryLock() } catch (_: OverlappingFileLockException) { null } catch (_: IOException) { return null }
                        if (lock != null) break
                        if (System.nanoTime() >= deadline) return null
                        Thread.sleep(25)
                    }
                    try { return body() } finally { try { lock.release() } catch (_: IOException) {} }
                }
            } finally {
                mutex.unlock()
            }
        }

        /** Copies the tree at [from] to [to] (which must not exist). */
        fun copyTree(from: File, to: File) {
            val source = from.toPath()
            val target = to.toPath()
            Files.walkFileTree(source, object : SimpleFileVisitor<Path>() {
                override fun preVisitDirectory(dir: Path, attrs: BasicFileAttributes): FileVisitResult {
                    Files.createDirectories(target.resolve(source.relativize(dir).toString()))
                    return FileVisitResult.CONTINUE
                }

                override fun visitFile(file: Path, attrs: BasicFileAttributes): FileVisitResult {
                    Files.copy(file, target.resolve(source.relativize(file).toString()), StandardCopyOption.COPY_ATTRIBUTES)
                    return FileVisitResult.CONTINUE
                }
            })
        }
    }
}
