package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.io.IOException
import java.nio.channels.FileChannel
import java.nio.channels.FileLock
import java.nio.channels.OverlappingFileLockException
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.FileVisitResult
import java.nio.file.LinkOption
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
 *
 * **The directory is not trusted.** It lives in the project tree, so anyone who can write there can plant entries in it.
 * Nothing here follows a symbolic link: only entries whose names are exactly what this class creates are listed for
 * deletion, links among them are skipped, a tree is deleted without entering links (a link inside it is unlinked), and
 * a target's real path must lie under the directory it is deleted from. `bundle.bin` is size-capped before it is read.
 */
internal class BundleStore(
    val root: File,
    private val keep: Int = DEFAULT_KEEP,
    private val maxBundleBytes: Long = defaultMaxBundleBytes(),
) {
    fun dir(compileKey: String): File {
        require(COMPILE_KEY.matches(compileKey)) { "not a compile key: $compileKey" }
        return File(root, "v${BundleCodec.VERSION}-$compileKey")
    }

    fun bundleFile(compileKey: String): File = File(dir(compileKey), BUNDLE_NAME)

    /** The classes of a bundle for reading; null when its directory or the classes are links or missing. */
    fun readableClassesDir(compileKey: String, id: String): File? {
        val dir = dir(compileKey).toPath()
        val classes = classesDir(compileKey, id).toPath()
        if (Files.isSymbolicLink(dir) || !Files.isDirectory(classes, LinkOption.NOFOLLOW_LINKS)) return null
        return classes.toFile()
    }

    fun classesDir(compileKey: String, id: String): File {
        require(BundleCodec.isSnapshotId(id)) { "not a snapshot id: $id" }
        return File(dir(compileKey), "$CLASSES_PREFIX$id")
    }

    private fun lockFile(compileKey: String): File = File(root, "v${BundleCodec.VERSION}-$compileKey.lock")

    /** Reads and checks the bundle of [compileKey]; throws [BundleRejected] when there is none or it cannot be used. */
    fun read(compileKey: String): DecodedBundle {
        val file = bundleFile(compileKey)
        val path = file.toPath()
        // As on the write side: a link in place of the bundle directory is not followed, it is a miss.
        if (Files.isSymbolicLink(dir(compileKey).toPath())) throw BundleRejected(BundleRejected.Reason.IO, "the bundle directory is a symbolic link")
        val bytes = try {
            val attrs = Files.readAttributes(path, BasicFileAttributes::class.java, LinkOption.NOFOLLOW_LINKS)
            if (!attrs.isRegularFile) throw BundleRejected(BundleRejected.Reason.IO, "bundle.bin is not a regular file")
            // The file is untrusted input: its size is checked before a byte of it is held in memory.
            if (attrs.size() > maxBundleBytes) {
                throw BundleRejected(BundleRejected.Reason.IO, "bundle.bin is ${attrs.size()} bytes, over the $maxBundleBytes limit")
            }
            val read = Files.newInputStream(path, LinkOption.NOFOLLOW_LINKS).use { it.readNBytes(maxBundleBytes.toInt() + 1) }
            if (read.size > maxBundleBytes) throw BundleRejected(BundleRejected.Reason.IO, "bundle.bin grew past the $maxBundleBytes limit")
            read
        } catch (_: NoSuchFileException) {
            throw BundleRejected(BundleRejected.Reason.MISSING, "no bundle for this compile key")
        } catch (e: IOException) {
            throw BundleRejected(BundleRejected.Reason.IO, "${e.javaClass.simpleName}: ${e.message}")
        } catch (_: OutOfMemoryError) {
            throw BundleRejected(BundleRejected.Reason.IO, "out of memory reading bundle.bin")
        }
        val decoded = BundleCodec.decode(bytes, compileKey)
        // Recency for eviction is use, not just writes.
        try { file.setLastModified(System.currentTimeMillis()) } catch (_: Exception) {}
        return decoded
    }

    /** A fresh empty directory next to where `classes-<id>` will go, for the caller to fill and hand to [publish]. */
    fun newStage(compileKey: String): File {
        val dir = dir(compileKey)
        ensureRealDirectory(dir.toPath())
        return File(dir, "$STAGE_PREFIX${UUID.randomUUID()}").also { Files.createDirectory(it.toPath()) }
    }

    /**
     * Publishes [bytes] as the bundle of [compileKey]. [staged] is a directory from [newStage] holding the snapshot
     * class files, which becomes `classes-<snapshotId>`; null when that directory was published already, in which case
     * nothing is published if it has gone (another process replaced it), since a bundle must not name classes that are
     * not there. Returns false when it was not published: the lock was not available in time, the classes vanished, or
     * [mayCommit] said no.
     *
     * [mayCommit] is asked, holding the monitor of [commitGate], before anything becomes visible, and the commit runs
     * inside that monitor: whoever flips what [mayCommit] reads while holding the same monitor knows that nothing is
     * published after it.
     *
     * `classes-<id>` of the new bundle and of the bundle it replaces are kept; every other class directory goes. The
     * replaced bundle is the one `bundle.bin` names under the lock, not the one with the newest timestamp.
     */
    fun publish(
        compileKey: String, bytes: ByteArray, snapshotId: String, staged: File?,
        commitGate: Any = Any(), mayCommit: () -> Boolean = { true },
    ): Boolean {
        require(BundleCodec.isSnapshotId(snapshotId)) { "not a snapshot id: $snapshotId" }
        Files.createDirectories(root.toPath())
        val result = withLock(lockFile(compileKey), LOCK_WAIT_MILLIS) {
            val dir = dir(compileKey)
            ensureRealDirectory(dir.toPath())
            val target = classesDir(compileKey, snapshotId)
            val replaced = referencedSnapshotId(compileKey)
            synchronized(commitGate) {
                if (!mayCommit()) return@withLock false
                if (staged != null) {
                    if (Files.exists(target.toPath(), LinkOption.NOFOLLOW_LINKS)) deleteTree(target.toPath(), dir.toPath())
                    moveAtomically(staged.toPath(), target.toPath())
                } else if (!Files.isDirectory(target.toPath(), LinkOption.NOFOLLOW_LINKS)) {
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
            }
            pruneClasses(dir, keepNames = setOfNotNull(target.name, replaced?.let { "$CLASSES_PREFIX$it" }))
            sweepLeftovers(dir)
            evict(except = compileKey)
            true
        }
        if (result != true && staged != null) deleteTree(staged.toPath(), staged.parentFile.toPath())
        return result == true
    }

    /** Deletes the bundles beyond the [keep] most recently used, never [except] and never one that is being written. */
    fun evict(except: String? = null) {
        val prefix = "v${BundleCodec.VERSION}-"
        // Older format versions are bundles too: a version bump must not leave the old ones behind forever. Only
        // names this class creates are touched, and a link is never a bundle.
        val dirs = root.listFiles { f -> BUNDLE_DIR.matches(f.name) && isRealDirectory(f.toPath()) }.orEmpty()
        val ranked = dirs.sortedByDescending { recency(it) }
        val protectedName = except?.let { prefix + it }
        var kept = 0
        for (dir in ranked) {
            if (dir.name == protectedName) { kept++; continue }
            if (kept < keep) { kept++; continue }
            val lock = File(root, dir.name + ".lock")
            withLock(lock, 0) {
                deleteTree(dir.toPath(), root.toPath())
                lock.delete()
            }
        }
    }

    private fun recency(dir: File): Long {
        val bundle = File(dir, BUNDLE_NAME).toPath()
        return modified(bundle).takeIf { it > 0 } ?: modified(dir.toPath())
    }

    // The snapshot id the current bundle.bin names, read from its header; null when there is none or it is unreadable.
    private fun referencedSnapshotId(compileKey: String): String? = try {
        val header = Files.newInputStream(bundleFile(compileKey).toPath(), LinkOption.NOFOLLOW_LINKS).use { it.readNBytes(HEADER_PEEK_BYTES) }
        BundleCodec.peekSnapshotId(header)
    } catch (_: Exception) { null }

    private fun pruneClasses(dir: File, keepNames: Set<String>) {
        for (f in dir.listFiles().orEmpty()) {
            if (!CLASSES_DIR.matches(f.name) || f.name in keepNames || !isRealDirectory(f.toPath())) continue
            deleteTree(f.toPath(), dir.toPath())
        }
    }

    // Staging directories and temp files of writers that died; one that is young may be another process's.
    private fun sweepLeftovers(dir: File) {
        val cutoff = System.currentTimeMillis() - LEFTOVER_MILLIS
        for (f in dir.listFiles().orEmpty()) {
            if (!(STAGE_NAME.matches(f.name) || TEMP_NAME.matches(f.name))) continue
            if (Files.isSymbolicLink(f.toPath())) continue
            if (modified(f.toPath()) < cutoff) deleteTree(f.toPath(), dir.toPath())
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
        private const val HEADER_PEEK_BYTES = 256

        // The names this class creates, matched whole. Anything else in the directory is not ours to delete.
        private const val UUID_PATTERN = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
        // The compile key is the hex of a SHA-256 (see FingerprintBuilder).
        private val COMPILE_KEY = Regex("[0-9a-f]{64}")
        private val BUNDLE_DIR = Regex("v\\d+-[0-9a-f]{64}")
        private val CLASSES_DIR = Regex("classes-[0-9a-f]{32}")
        private val STAGE_NAME = Regex("\\.stage-$UUID_PATTERN")
        private val TEMP_NAME = Regex("\\.bundle-$UUID_PATTERN")

        /**
         * The directory a request may persist bundles in, or null (persistence off for that request) when it is not
         * `<something>/.krit/fir-incremental` once canonical. The path comes over the wire, and everything under it is
         * created and deleted by this process, so it must not be an arbitrary directory.
         */
        fun trustedRoot(path: String?, warn: (String) -> Unit = { System.err.println("krit-fir: $it") }): File? {
            if (path.isNullOrBlank()) return null
            val canonical = try { File(path).canonicalFile } catch (_: IOException) {
                warn("ignoring incrementalDir $path: it cannot be resolved; persistence is off for this request")
                return null
            }
            if (!File(path).isAbsolute || canonical.name != "fir-incremental" || canonical.parentFile?.name != ".krit") {
                warn("ignoring incrementalDir $path: it is not an absolute .krit/fir-incremental directory; persistence is off for this request")
                return null
            }
            return canonical
        }

        /** The largest `bundle.bin` that is read: 512 MiB, and never more than a quarter of the heap. */
        fun defaultMaxBundleBytes(): Long = minOf(512L * 1024 * 1024, Runtime.getRuntime().maxMemory() / 4)

        // One mutex per lock file in this JVM: see the class comment.
        private val mutexes = ConcurrentHashMap<String, ReentrantLock>()

        /** Runs [body] holding the lock on [lockFile], waiting up to [waitMillis]; null if the lock was not obtained. */
        fun <T> withLock(lockFile: File, waitMillis: Long, body: () -> T): T? {
            val mutex = mutexes.computeIfAbsent(try { lockFile.canonicalPath } catch (_: IOException) { lockFile.absolutePath }) { ReentrantLock() }
            if (!mutex.tryLock(waitMillis, TimeUnit.MILLISECONDS)) return null
            try {
                // A planted FIFO or device would block the open for ever: only a regular file (or none) is a lock.
                val existing = try { Files.readAttributes(lockFile.toPath(), BasicFileAttributes::class.java, LinkOption.NOFOLLOW_LINKS) }
                    catch (_: NoSuchFileException) { null } catch (_: IOException) { return null }
                if (existing != null && !existing.isRegularFile) return null
                val channel = try {
                    FileChannel.open(lockFile.toPath(), StandardOpenOption.CREATE, StandardOpenOption.WRITE, LinkOption.NOFOLLOW_LINKS)
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

        private fun isRealDirectory(path: Path) = Files.isDirectory(path, LinkOption.NOFOLLOW_LINKS)

        private fun modified(path: Path): Long = try {
            Files.getLastModifiedTime(path, LinkOption.NOFOLLOW_LINKS).toMillis()
        } catch (_: IOException) { 0L }

        // Creates [path] if needed; a link in its place is refused, so nothing is ever written through one.
        private fun ensureRealDirectory(path: Path) {
            if (Files.isSymbolicLink(path)) throw IOException("$path is a symbolic link")
            Files.createDirectories(path)
        }

        /**
         * Deletes [target] and everything under it without following a link, and only when its real path lies strictly
         * under [within]'s. A link at [target] is not deleted (false); a link inside the tree is unlinked, never entered.
         */
        internal fun deleteTree(target: Path, within: Path): Boolean {
            try {
                if (Files.isSymbolicLink(target)) return false
                val real = target.toRealPath()
                val bound = within.toRealPath()
                if (real == bound || !real.startsWith(bound)) return false
                // walkFileTree does not follow links unless asked to (FOLLOW_LINKS): a link comes through visitFile.
                Files.walkFileTree(target, object : SimpleFileVisitor<Path>() {
                    override fun visitFile(file: Path, attrs: BasicFileAttributes): FileVisitResult {
                        Files.deleteIfExists(file)
                        return FileVisitResult.CONTINUE
                    }

                    override fun visitFileFailed(file: Path, exc: IOException): FileVisitResult = FileVisitResult.CONTINUE

                    override fun postVisitDirectory(dir: Path, exc: IOException?): FileVisitResult {
                        try { Files.deleteIfExists(dir) } catch (_: IOException) {}
                        return FileVisitResult.CONTINUE
                    }
                })
                return !Files.exists(target, LinkOption.NOFOLLOW_LINKS)
            } catch (_: IOException) {
                return false
            }
        }

        /**
         * Copies the tree at [from] to [to] (which must not exist). Links are never followed and never copied: a link
         * in the tree is skipped, so the copy holds only regular files and directories.
         */
        fun copyTree(from: File, to: File) {
            val source = from.toPath()
            val target = to.toPath()
            if (Files.isSymbolicLink(source)) throw IOException("$source is a symbolic link")
            Files.walkFileTree(source, object : SimpleFileVisitor<Path>() {
                override fun preVisitDirectory(dir: Path, attrs: BasicFileAttributes): FileVisitResult {
                    Files.createDirectories(target.resolve(source.relativize(dir).toString()))
                    return FileVisitResult.CONTINUE
                }

                override fun visitFile(file: Path, attrs: BasicFileAttributes): FileVisitResult {
                    if (attrs.isSymbolicLink || !attrs.isRegularFile) return FileVisitResult.CONTINUE
                    Files.copy(
                        file, target.resolve(source.relativize(file).toString()),
                        StandardCopyOption.COPY_ATTRIBUTES, LinkOption.NOFOLLOW_LINKS,
                    )
                    return FileVisitResult.CONTINUE
                }
            })
        }
    }
}
