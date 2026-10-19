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
 * Module name of snapshot and slice compiles. A slice only sees the module's own binaries (and so `internal`,
 * sealed hierarchies and facades resolve as in a full compile) when it is compiled under the module name the
 * snapshot was built with; the name also names `META-INF/<name>.kotlin_module` in the snapshot.
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
    /** [result] is today's per-file shape for the [compiled] files (canonical paths). */
    class Checked(val result: dev.jasonpearson.krit.fir.runner.BatchResult, val compiled: Set<String>) : SliceOutcome

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

    @Synchronized fun classFileCount(): Int = outputs.values.sumOf { it.size }

    @Synchronized fun hasOutputsFor(source: String): Boolean = outputs.containsKey(canonicalPath(source))

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
                // Recorded first: whatever was moved before a failure is still restored by unpark.
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
 * `os.UserCacheDir()/krit`), never the user's project. Each snapshot gets its own directory named after the
 * process, so a snapshot left behind by a daemon that died is swept by the next one.
 */
internal object SnapshotStorage {
    private const val DIR_NAME = "fir-snapshots"
    private val sequence = AtomicInteger()
    private val pid = ProcessHandle.current().pid()
    private val hookInstalled = java.util.concurrent.atomic.AtomicBoolean()
    private val used = java.util.concurrent.ConcurrentHashMap.newKeySet<File>()

    /** `KRIT_FIR_CACHE_DIR` (or `-Dkrit.fir.cache.dir`) wins; otherwise the per-user cache dir plus `krit`. */
    fun cacheDir(): File {
        System.getProperty("krit.fir.cache.dir")?.takeIf { it.isNotBlank() }?.let { return File(it) }
        System.getenv("KRIT_FIR_CACHE_DIR")?.takeIf { it.isNotBlank() }?.let { return File(it) }
        val home = System.getProperty("user.home").orEmpty()
        val os = System.getProperty("os.name").orEmpty().lowercase()
        val base = when {
            os.contains("win") -> System.getenv("LOCALAPPDATA")?.takeIf { it.isNotBlank() }?.let(::File)
            os.contains("mac") -> home.takeIf { it.isNotBlank() }?.let { File(it, "Library/Caches") }
            else -> System.getenv("XDG_CACHE_HOME")?.takeIf { it.isNotBlank() }?.let(::File)
                ?: home.takeIf { it.isNotBlank() }?.let { File(it, ".cache") }
        }
        return if (base != null) File(base, "krit")
        else File(System.getProperty("java.io.tmpdir"), "krit-${System.getProperty("user.name").orEmpty()}")
    }

    /** Creates an empty private directory for one snapshot under [cacheDir] (or [parent]'s `fir-snapshots`). */
    fun newSnapshotRoot(parent: File? = null): File {
        val snapshots = File(parent ?: cacheDir(), DIR_NAME)
        createPrivateDirectories(snapshots.toPath())
        sweepDeadProcesses(snapshots)
        used += snapshots
        installCleanupHook()
        val root = File(snapshots, "p$pid-${sequence.incrementAndGet()}-${java.lang.Long.toHexString(System.nanoTime())}")
        createPrivateDirectories(root.toPath())
        return root
    }

    private fun createPrivateDirectories(path: Path) {
        try {
            Files.createDirectories(path, PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")))
        } catch (_: UnsupportedOperationException) {
            Files.createDirectories(path)
        }
    }

    // Directories "p<pid>-..." of processes that no longer run.
    private fun sweepDeadProcesses(snapshots: File) {
        for (child in snapshots.listFiles().orEmpty()) {
            val owner = child.name.removePrefix("p").substringBefore('-').toLongOrNull() ?: continue
            if (owner != pid && !ProcessHandle.of(owner).isPresent) child.deleteRecursively()
        }
    }

    private fun installCleanupHook() {
        if (!hookInstalled.compareAndSet(false, true)) return
        Runtime.getRuntime().addShutdownHook(Thread {
            for (snapshots in used) for (child in snapshots.listFiles().orEmpty()) {
                if (child.name.startsWith("p$pid-")) child.deleteRecursively()
            }
        })
    }
}
