@file:OptIn(org.jetbrains.kotlin.compiler.plugin.ExperimentalCompilerApi::class)
@file:Suppress("DEPRECATION", "DEPRECATION_ERROR")

package dev.jasonpearson.krit.fir.incremental

import org.jetbrains.kotlin.cli.common.messages.OutputMessageUtil
import org.jetbrains.kotlin.cli.jvm.compiler.VfsBasedProjectEnvironment
import org.jetbrains.kotlin.cli.jvm.compiler.legacy.pipeline.IncrementalCompilationComponentsWithCustomScope
import org.jetbrains.kotlin.fir.session.environment.AbstractProjectFileSearchScope
import org.jetbrains.kotlin.load.kotlin.incremental.components.IncrementalCache
import org.jetbrains.kotlin.modules.TargetId
import java.io.File
import java.io.IOException
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.nio.file.attribute.PosixFilePermissions
import java.util.concurrent.atomic.AtomicInteger

/**
 * Module name of snapshot and slice compiles. It names `META-INF/<name>.kotlin_module` in the snapshot, which the
 * slice's incremental cache serves, and keeps the module-name-dependent parts of class files (the mangled names of
 * `internal` members) the same in the snapshot and in anything a slice compiles. Both compiles use it so they
 * cannot disagree; no frontend construct in the test suite fails without it (the mechanism that makes `internal`
 * and sealed hierarchies resolve is the incremental-compilation scope, not the name).
 */
internal const val SNAPSHOT_MODULE_NAME = "krit-incremental"

/** Why a slice compile cannot be used, so the planner serves the request with a full compile instead. */
internal enum class SliceFallback(val message: String) {
    /** No snapshot was built: the first compile has not run, it had compiler errors, or building the snapshot failed. */
    NO_SNAPSHOT("no binary snapshot of the module"),

    /** The module uses `expect`/`actual` (or compiles as multiplatform): K2 never writes `expect` declarations to class files. */
    EXPECT_ACTUAL("the module uses expect/actual declarations"),

    /** The compiler, plugin, classpath or settings changed since the snapshot was built. */
    STATE_CHANGED("the compiler facts changed since the snapshot was built"),

    /** The slice compile threw or crashed. The snapshot and the recorded facts are untouched. */
    SLICE_FAILED("the slice compile failed"),
}

/** What a slice check came to: verdicts for the compiled files, or the reason the planner must run a full check. */
internal sealed interface SliceOutcome {
    /**
     * [result] is today's per-file shape for the [compiled] files (canonical paths): the requested files plus
     * [added], which the slice needed to be sound (binary-stale files, the inheritors of a sealed parent).
     */
    class Checked(
        val result: dev.jasonpearson.krit.fir.runner.BatchResult,
        val compiled: Set<String>,
        val added: Set<String> = emptySet(),
    ) : SliceOutcome

    class Fallback(val reason: SliceFallback, val cause: Throwable? = null) : SliceOutcome
}

/** The two points a test can make fail on purpose; null in production. */
internal enum class IncrementalStage {
    /** The backend run that writes the snapshot, after a clean frontend. */
    SNAPSHOT_BACKEND,

    /** A slice compile, while the stale outputs are parked. */
    SLICE_COMPILE,
}

internal object IncrementalTestHook {
    @Volatile var onStage: ((IncrementalStage) -> Unit)? = null

    /** Makes the snapshot marker believe the compiler's message buffer is not available. */
    @Volatile var hideMessageBuffer = false
}

/**
 * The module's binary snapshot: the class files of a clean full compile with the backend, the source -> output map
 * from the compiler's `-Xreport-output-files` messages and the module mapping.
 *
 * The directory [dir] is the only thing a slice compile reads. [parked] moves the outputs of the sources being
 * recompiled aside for the duration of the compile, so a top-level function never exists in source and binary at
 * once, and puts them back whether the compile succeeds or throws. The snapshot lives in [root], a directory this
 * snapshot owns.
 */
internal class Snapshot(val root: File) {
    val dir = File(root, "classes")
    private val aside = File(root, "aside")

    private val outputs = HashMap<String, List<String>>() // canonical source -> class file paths relative to dir
    private val parkedOutputs = HashMap<String, List<String>>()
    @Volatile private var mapping: ByteArray? = null

    /** True when parking or restoring a file failed: the directory no longer matches [outputs] and must not be used. */
    @Volatile var corrupt = false
        private set

    @Synchronized fun fileCount(): Int = outputs.size

    fun moduleMapping(): ByteArray? = mapping

    /** Takes over the compile's [reported] outputs (canonical source -> absolute output paths) and its module mapping. */
    @Synchronized
    fun adopt(reported: Map<String, List<String>>) {
        outputs.clear()
        val base = dir.toPath().toAbsolutePath().normalize()
        val canonicalBase = try { dir.canonicalFile.toPath() } catch (_: IOException) { base }
        for ((source, paths) in reported) {
            outputs[source] = paths.filter { it.endsWith(".class") }.mapNotNull { relativeTo(it, base, canonicalBase) }
        }
        mapping = File(dir, "META-INF/$SNAPSHOT_MODULE_NAME.kotlin_module").takeIf { it.isFile }?.readBytes()
    }

    private fun relativeTo(path: String, base: Path, canonicalBase: Path): String? {
        val p = File(path).toPath().toAbsolutePath().normalize()
        if (p.startsWith(base)) return base.relativize(p).toString()
        val c = try { File(path).canonicalFile.toPath() } catch (_: IOException) { return null }
        return if (c.startsWith(canonicalBase)) canonicalBase.relativize(c).toString() else null
    }

    /**
     * Runs [block] with the outputs of [sources] moved out of the snapshot, then restores them, also when [block]
     * throws. A source without outputs (new since the snapshot) has nothing to move.
     */
    fun <T> parked(sources: Collection<String>, block: () -> T): T {
        park(sources)
        val result = try {
            block()
        } catch (t: Throwable) {
            try { unpark() } catch (restore: Throwable) { t.addSuppressed(restore) }
            throw t
        }
        unpark()
        return result
    }

    @Synchronized
    fun park(sources: Collection<String>) {
        try {
            for (source in sources) {
                val rels = outputs[canonicalPath(source)] ?: continue
                // Recorded before the moves, so that if one of them fails the files moved so far are the ones
                // `unpark` (run by `parked`, in `finally`) tries to put back; it skips files that never moved.
                parkedOutputs[canonicalPath(source)] = rels
                for (rel in rels) move(File(dir, rel), File(aside, rel))
            }
        } catch (e: IOException) {
            corrupt = true
            throw e
        }
    }

    @Synchronized
    fun unpark() {
        var failure: IOException? = null
        for ((_, rels) in parkedOutputs) {
            for (rel in rels) {
                try { move(File(aside, rel), File(dir, rel)) } catch (e: IOException) { failure = failure ?: e }
            }
        }
        parkedOutputs.clear()
        if (failure != null) {
            corrupt = true
            throw failure
        }
    }

    /** Deletes the snapshot from disk. */
    @Synchronized
    fun delete() {
        outputs.clear(); parkedOutputs.clear(); mapping = null
        SnapshotStorage.release(root)
        root.deleteRecursively()
    }

    private fun move(from: File, to: File) {
        if (!from.exists()) return
        to.parentFile.mkdirs()
        Files.move(from.toPath(), to.toPath(), StandardCopyOption.REPLACE_EXISTING)
    }

    companion object {
        /** Canonical source -> output files of the compiler's OUTPUT messages (`-Xreport-output-files`). */
        fun parseOutputs(messages: Collection<String>): Map<String, List<String>> {
            val out = HashMap<String, MutableList<String>>()
            for (message in messages) {
                val output = OutputMessageUtil.parseOutputMessage(message) ?: continue
                val file = output.outputFile ?: continue
                for (source in output.sourceFiles) out.getOrPut(canonicalPath(source.path)) { ArrayList() }.add(file.path)
            }
            return out
        }
    }
}

/** What the frontend reads from the incremental cache of the module's previous compile: only the module mapping. */
private class SnapshotCache(private val snapshot: Snapshot) : IncrementalCache {
    override fun getObsoletePackageParts(): Collection<String> = emptyList()
    override fun getObsoleteMultifileClasses(): Collection<String> = emptyList()
    override fun getStableMultifileFacadeParts(facadeInternalName: String): Collection<String>? = null
    override fun getPackagePartData(partInternalName: String) = null
    override fun getModuleMappingData(): ByteArray? = snapshot.moduleMapping()
    override fun getClassFilePath(internalClassName: String): String = File(snapshot.dir, "$internalClassName.class").path
    override fun close() {}
}

/**
 * The incremental-compilation components the stock CLI frontend reads from `Services`: the snapshot directory
 * becomes the "binaries of the previous compilation" scope of the same module (carrying the source module's own
 * module data) and is subtracted from the libraries scope.
 */
internal class SnapshotComponents(private val snapshot: Snapshot) : IncrementalCompilationComponentsWithCustomScope {
    override fun getIncrementalCache(target: TargetId): IncrementalCache = SnapshotCache(snapshot)
    override fun createSearchScope(projectEnvironment: VfsBasedProjectEnvironment): AbstractProjectFileSearchScope =
        projectEnvironment.getSearchScopeByDirectories(setOf(snapshot.dir))
}

/**
 * Where snapshots live: a krit-owned directory in the per-user cache (the same place as the Go side's
 * `os.UserCacheDir()/krit`), never the user's project.
 *
 * Each snapshot has its own directory, and its owner holds an OS file lock on `.lock` inside it for as long as the
 * snapshot exists. A directory whose lock can be taken has no owner any more (the process died, however it died)
 * and is swept by the next process to create a snapshot. Liveness is the lock and not a process id, so processes in
 * different pid namespaces that share a cache volume (containers) never delete each other's live snapshots. A
 * directory without a lock file is one whose owner has not made it yet, or an interrupted one: it is swept only
 * when it is older than [LOCKLESS_GRACE_MILLIS].
 */
internal object SnapshotStorage {
    private const val DIR_NAME = "fir-snapshots"
    private const val LOCK_NAME = ".lock"
    const val LOCKLESS_GRACE_MILLIS = 10 * 60 * 1000L
    private const val MAX_CREATE_ATTEMPTS = 5

    // Serializes, within this process, the sweep and the stretch of a creation from opening `.lock` to owning it.
    // Both open channels on the same file, and in one JVM that goes wrong two ways: a sweep that holds the lock makes
    // the creator's `tryLock` throw OverlappingFileLockException, and a sweep that merely opens and closes a channel
    // after the creator locked the file drops the creator's lock (on Linux closing any channel on a file does).
    // Other processes need no such guard: their locks and close() calls are theirs alone.
    private val guard = Any()

    private val sequence = AtomicInteger()
    private val pid = ProcessHandle.current().pid()
    private val hookInstalled = java.util.concurrent.atomic.AtomicBoolean()

    // This process's snapshot roots (canonical) and the lock channels it holds on them. Closing a channel releases
    // its lock, and on Linux closing ANY channel on the file does, so nothing but the owner may open `.lock`: the
    // sweep must recognise this process's directories whatever way its path is spelled.
    private val held = java.util.concurrent.ConcurrentHashMap<File, java.nio.channels.FileChannel>()

    /** `KRIT_FIR_CACHE_DIR` (or `-Dkrit.fir.cache.dir`) wins; otherwise the per-user cache dir plus `krit`. */
    fun cacheDir(): File = cacheDirFor(
        property = System.getProperty("krit.fir.cache.dir"),
        env = System.getenv("KRIT_FIR_CACHE_DIR"),
        os = System.getProperty("os.name").orEmpty(),
        home = System.getProperty("user.home").orEmpty(),
        xdgCacheHome = System.getenv("XDG_CACHE_HOME"),
        localAppData = System.getenv("LOCALAPPDATA"),
        tmpDir = System.getProperty("java.io.tmpdir").orEmpty(),
        user = System.getProperty("user.name").orEmpty(),
    )

    /** The precedence of [cacheDir] over explicit inputs: property, then env, then the OS default. */
    fun cacheDirFor(
        property: String?, env: String?, os: String, home: String, xdgCacheHome: String?, localAppData: String?,
        tmpDir: String, user: String,
    ): File {
        property?.takeIf { it.isNotBlank() }?.let { return File(it) }
        env?.takeIf { it.isNotBlank() }?.let { return File(it) }
        val system = os.lowercase()
        val base = when {
            system.contains("win") -> localAppData?.takeIf { it.isNotBlank() }?.let(::File)
            system.contains("mac") -> home.takeIf { it.isNotBlank() }?.let { File(it, "Library/Caches") }
            else -> xdgCacheHome?.takeIf { it.isNotBlank() }?.let(::File)
                ?: home.takeIf { it.isNotBlank() }?.let { File(it, ".cache") }
        }
        return if (base != null) File(base, "krit") else File(tmpDir, "krit-$user")
    }

    /**
     * Creates an empty private directory for one snapshot under [parent] (default [cacheDir]) and locks it for this
     * process. Sweeps the directories of dead owners first.
     */
    fun newSnapshotRoot(parent: File? = null): File {
        val snapshots = File(parent ?: cacheDir(), DIR_NAME)
        createPrivateDirectories(snapshots.toPath())
        sweepDead(snapshots)
        installCleanupHook()
        // Between creating the lock file and locking it a concurrent sweep may delete the directory (nothing is
        // locked yet): the directory is checked again once locked, and a fresh one is made if it is gone.
        repeat(MAX_CREATE_ATTEMPTS) {
            val root = File(snapshots, "p$pid-${sequence.incrementAndGet()}-${java.lang.Long.toHexString(System.nanoTime())}")
            createPrivateDirectories(root.toPath())
            val owned = synchronized(guard) {
                val channel = try {
                    java.nio.channels.FileChannel.open(
                        File(root, LOCK_NAME).toPath(),
                        java.nio.file.StandardOpenOption.CREATE, java.nio.file.StandardOpenOption.WRITE,
                    )
                } catch (_: IOException) { return@synchronized false }
                // Registered before locking: from then on this process's sweep leaves the file alone.
                val key = root.canonicalFile
                held[key] = channel
                val locked = try {
                    // A lock another process (or, defensively, this JVM) holds is not ours: try a fresh directory.
                    val lock = try { channel.tryLock() } catch (_: java.nio.channels.OverlappingFileLockException) { null }
                    lock != null && root.isDirectory && File(root, LOCK_NAME).isFile
                } catch (e: Throwable) {
                    held.remove(key)
                    channel.close()
                    throw e
                }
                if (!locked) {
                    held.remove(key)
                    channel.close()
                }
                locked
            }
            if (owned) return root
            root.deleteRecursively()
        }
        throw IOException("could not create a snapshot directory under $snapshots")
    }

    /** Releases this process's lock on [root] (the directory is deleted by the caller). */
    fun release(root: File) {
        try { held.remove(root.canonicalFile)?.close() } catch (_: IOException) {}
    }

    /**
     * Deletes the snapshot directories under [snapshots] that nobody owns. Returns how many it deleted. A directory
     * is kept when this process holds it, when another process holds its lock, or when it has no lock file yet and
     * is younger than [LOCKLESS_GRACE_MILLIS].
     */
    fun sweepDead(snapshots: File): Int = synchronized(guard) {
        var removed = 0
        for (child in snapshots.listFiles().orEmpty()) {
            if (!child.isDirectory || held.containsKey(child.canonicalFile)) continue
            val lockFile = File(child, LOCK_NAME)
            if (!lockFile.isFile) {
                if (System.currentTimeMillis() - child.lastModified() > LOCKLESS_GRACE_MILLIS) {
                    child.deleteRecursively(); removed++
                }
                continue
            }
            val channel = try {
                java.nio.channels.FileChannel.open(lockFile.toPath(), java.nio.file.StandardOpenOption.WRITE)
            } catch (_: IOException) { continue }
            try {
                // null: another process holds it; OverlappingFileLockException: this JVM does.
                val lock = try { channel.tryLock() } catch (_: java.nio.channels.OverlappingFileLockException) { null }
                if (lock != null) { child.deleteRecursively(); removed++ }
            } catch (_: IOException) {
            } finally {
                try { channel.close() } catch (_: IOException) {}
            }
        }
        removed
    }

    /** Deletes this process's snapshot directories: what the shutdown hook runs. Other processes' are not touched. */
    fun removeOwned() {
        for (root in held.keys.toList()) {
            release(root)
            root.deleteRecursively()
        }
    }

    private fun createPrivateDirectories(path: Path) {
        try {
            Files.createDirectories(path, PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")))
        } catch (_: UnsupportedOperationException) {
            Files.createDirectories(path)
        }
    }

    private fun installCleanupHook() {
        if (!hookInstalled.compareAndSet(false, true)) return
        Runtime.getRuntime().addShutdownHook(Thread { removeOwned() })
    }
}
