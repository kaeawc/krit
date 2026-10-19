package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.security.MessageDigest
import java.util.HexFormat
import java.util.concurrent.ConcurrentHashMap

/**
 * What one whole-module compile recorded: lookups and declaration surfaces per canonical source path, plus
 * content hashes. [compileKey] identifies the compiler facts (lookups, surfaces); [verdictKey] identifies what
 * a verdict additionally depends on: rules, rule configs, test-file classification, scan paths and SDK levels.
 * It does not cover which files were requested. Step 4 must derive per-file verdict keys from it, because
 * testFiles, scanPaths and sdkLevels are per-file maps.
 *
 * Recording is best-effort: a failure while recording poisons it with [fail], and a poisoned recording is
 * never committed.
 */
internal class CompileRecording(val compileKey: String, val verdictKey: String, val contentHashes: Map<String, String>) {
    val lookups = CompileLookups()
    val surfaces = ConcurrentHashMap<String, FileSurface>()

    @Volatile var failure: Throwable? = null
        private set
    val failed: Boolean get() = failure != null

    fun fail(cause: Throwable) {
        if (failure == null) failure = cause
    }
}

/**
 * Publishes the active [CompileRecording] to the checkers, scoped to the compiling thread like
 * [dev.jasonpearson.krit.fir.oracle.OracleCollectorRegistry].
 */
internal object CompileRecordingRegistry {
    private val active = ThreadLocal<CompileRecording?>()
    fun begin(recording: CompileRecording) = active.set(recording)
    fun current(): CompileRecording? = active.get()
    fun end() = active.remove()
}

/** `KRIT_FIR_INCREMENTAL=1` turns recording on for sessions that do not choose explicitly. */
internal fun incrementalRecordingFromEnv(): Boolean = System.getenv("KRIT_FIR_INCREMENTAL") == "1"

/** Lower-case hex of the first [length] bytes of [bytes]. */
internal fun hex(bytes: ByteArray, length: Int = bytes.size): String = HexFormat.of().formatHex(bytes, 0, length)

internal fun sha1Hex(bytes: ByteArray): String = hex(MessageDigest.getInstance("SHA-1").digest(bytes))

/** Content hash of each source file, keyed by canonical path; unreadable files are left out. */
internal fun contentHashesOf(paths: Collection<String>): Map<String, String> {
    val out = LinkedHashMap<String, String>()
    for (path in paths) {
        try { out[canonicalPath(path)] = sha1Hex(File(path).readBytes()) } catch (_: Exception) {}
    }
    return out
}

/** Digest of a list of labelled parts, added in a fixed order; maps and sets are rendered in sorted order. */
internal class FingerprintBuilder {
    private val digest = MessageDigest.getInstance("SHA-256")

    fun add(label: String, value: Any?): FingerprintBuilder {
        digest.update(label.toByteArray()); digest.update(0)
        digest.update(render(value).toByteArray()); digest.update(0)
        return this
    }

    /**
     * Path, size and modification time: cheap, and changes whenever the classpath entry is rebuilt. A directory
     * (compiled classes of an upstream module) is walked and each regular file's relative path, size and
     * modification time is included, sorted; the root's own mtime does not change when nested files are rewritten.
     * Contents are never read.
     */
    fun addFileIdentity(label: String, path: String): FingerprintBuilder {
        val f = File(path)
        if (!f.isDirectory) return add(label, listOf(canonicalPath(path), if (f.isFile) f.length() else -1L, f.lastModified()))
        val root = f.toPath()
        val entries = f.walkTopDown().filter { it.isFile }
            .map { "${root.relativize(it.toPath())}|${it.length()}|${it.lastModified()}" }.sorted().toList()
        return add(label, listOf(canonicalPath(path), "dir", entries))
    }

    fun build(): String = hex(digest.digest())

    private fun render(value: Any?): String = when (value) {
        null -> "null"
        is Map<*, *> -> value.entries.map { render(it.key) to it.value }.sortedBy { it.first }
            .joinToString(",", "{", "}") { "${it.first}=${render(it.second)}" }
        is Set<*> -> value.map { render(it) }.sorted().joinToString(",", "[", "]")
        is Iterable<*> -> value.joinToString(",", "(", ")") { render(it) }
        else -> value.toString()
    }
}

/**
 * How a file's declaration surface moved between the two latest commits. [previous] is the file's surface
 * before the move (null for a file new in this commit or when nothing changed), so a caller can see the old
 * supertypes of a changed or removed class.
 */
internal class SurfaceChange(val symbols: Set<Sym>, val scopes: Set<String>, val previous: FileSurface? = null) {
    val isEmpty: Boolean get() = symbols.isEmpty() && scopes.isEmpty()

    companion object {
        fun of(old: FileSurface?, new: FileSurface?): SurfaceChange {
            val (syms, scopes) = FileSurface.diff(old, new)
            val change = SurfaceChange(syms, scopes, old)
            return if (change.isEmpty) SurfaceChange(emptySet(), emptySet(), null) else change
        }
    }
}

/**
 * Incremental facts of one [dev.jasonpearson.krit.fir.runner.AnalysisSession]: per-file content hashes and
 * declaration-surface hashes, and the reverse index of what each file's last compile looked up. Nothing
 * consumes it yet; the slice-compile planner will.
 *
 * Every [commit] is a whole-module compile and replaces the per-file facts. The facts are compiler facts, so a
 * commit under a different [compileKey] (compiler, sources, classpath, settings) resets the state first. The
 * [verdictKey] (rules, rule configs, test-file classification, scan paths and SDK levels; not the requested
 * files) only records which verdict inputs the latest commit used; a verdict cache keyed by it can sit on top
 * without losing lookups or surfaces when the rules change. Step 4 must derive per-file verdict keys, because
 * testFiles, scanPaths and sdkLevels are per-file maps.
 *
 * A slice compile (see [commitSlice]) *merges*: it replaces the facts of the files it compiled and keeps the rest.
 *
 * The state also owns the module's binary [snapshot] (attached after a clean full compile, see [attachSnapshot])
 * and tracks which files the snapshot's binaries no longer describe ([binaryStale]). A slice compile reads every
 * file outside its source set from the snapshot, so a planner must keep the binary-stale files in every slice.
 */
internal open class IncrementalState {
    @Volatile var compileKey: String? = null
        private set
    @Volatile var verdictKey: String? = null
        private set

    private val surfaces = HashMap<String, FileSurface>()
    private val contentHashes = HashMap<String, String>()
    private val index = LookupIndex()
    private var changes = HashMap<String, SurfaceChange>()
    private var commits = 0

    // Declaration indexes over the latest commit, for the structural rules lookups cannot see.
    private val classFile = HashMap<String, String>()
    private val declFiles = HashMap<Sym, MutableSet<String>>()
    private val subclassFiles = HashMap<String, MutableSet<String>>()

    // The binary snapshot of the module and what became of it since it was built.
    private var snapshotOrNull: Snapshot? = null
    private var noSnapshotReason = SliceFallback.NO_SNAPSHOT
    private val snapshotSurfaceHashes = HashMap<String, String>() // file -> surface hash its binary was built from
    private val binaryStaleFiles = HashSet<String>()
    private val removedFiles = HashSet<String>() // had a binary in the snapshot, deleted since
    private var sliceFailure: Throwable? = null

    /** Number of commits since the last reset. */
    val generation: Int @Synchronized get() = commits

    /** Replaces the whole state with a whole-module compile. Drops the snapshot: [attachSnapshot] a new one after. */
    @Synchronized
    open fun commit(recording: CompileRecording) {
        val previous = if (compileKey == recording.compileKey) HashMap(surfaces) else null
        if (previous == null) reset()
        releaseSnapshot()
        compileKey = recording.compileKey
        verdictKey = recording.verdictKey
        val next = HashMap<String, SurfaceChange>()
        if (previous != null) {
            for (file in previous.keys + recording.surfaces.keys) {
                next[file] = SurfaceChange.of(previous[file], recording.surfaces[file])
            }
        }
        changes = next
        surfaces.clear(); surfaces.putAll(recording.surfaces)
        contentHashes.clear(); contentHashes.putAll(recording.contentHashes)
        // Rebuilt from this recording alone, so a file that lost all its lookups keeps none.
        index.clear()
        for (file in recording.surfaces.keys) index.update(file, recording.lookups.byFile[file] ?: emptySet())
        rebuildDeclarationIndexes()
        commits++
    }

    /**
     * Merges a slice compile into the state: replaces the facts of the [compiled] files (content hash, surface,
     * lookups, declaration indexes) with [recording]'s and keeps every other file's. [removed] files, deleted since
     * the last commit, lose theirs. [errorFiles] are the compiled files that had a compiler error.
     *
     * [changeOf] describes the move of each compiled and removed file in this commit (with its previous surface)
     * and reports every other file as unchanged. It covers the latest commit only: a planner that runs several
     * waves of slices must consume the changes of each wave before the next one. Binary staleness is updated for the compiled files: see
     * [binaryStale]. [compileKey] stays; [verdictKey] becomes the slice's.
     */
    @Synchronized
    open fun commitSlice(
        recording: CompileRecording,
        compiled: Collection<String>,
        removed: Collection<String> = emptyList(),
        errorFiles: Collection<String> = emptyList(),
    ) {
        check(compileKey != null && compileKey == recording.compileKey) {
            "a slice recorded under a different compile key cannot be merged"
        }
        val compiledSet = compiled.mapTo(HashSet()) { canonicalPath(it) }
        val removedSet = removed.mapTo(HashSet()) { canonicalPath(it) }.also { it -= compiledSet }
        val errored = errorFiles.mapTo(HashSet()) { canonicalPath(it) }
        verdictKey = recording.verdictKey

        val next = HashMap<String, SurfaceChange>()
        for (file in surfaces.keys) next[file] = UNCHANGED
        for (file in compiledSet) next[file] = SurfaceChange.of(surfaces[file], recording.surfaces[file])
        for (file in removedSet) next[file] = SurfaceChange.of(surfaces[file], null)
        changes = next

        for (file in removedSet) {
            surfaces.remove(file); contentHashes.remove(file); index.remove(file)
            binaryStaleFiles.remove(file)
            if (file in snapshotSurfaceHashes) removedFiles += file
        }
        for (file in compiledSet) {
            recording.surfaces[file]?.let { surfaces[file] = it } ?: surfaces.remove(file)
            recording.contentHashes[file]?.let { contentHashes[file] = it } ?: contentHashes.remove(file)
            index.update(file, recording.lookups.byFile[file] ?: emptySet())
            removedFiles.remove(file)
            if (snapshotOrNull != null) {
                val builtFrom = snapshotSurfaceHashes[file]
                val now = surfaces[file]?.let(::hashOf)
                if (file in errored || builtFrom == null || now != builtFrom) binaryStaleFiles += file else binaryStaleFiles -= file
            }
        }
        rebuildDeclarationIndexes()
        commits++
    }

    private fun rebuildDeclarationIndexes() {
        classFile.clear(); declFiles.clear(); subclassFiles.clear()
        for ((file, surface) in surfaces) {
            for (key in surface.decls.keys) declFiles.getOrPut(plain(key)) { HashSet() }.add(file)
            for ((cls, supers) in surface.supers) {
                classFile[cls] = file
                for (s in supers) subclassFiles.getOrPut(erase(s)) { HashSet() }.add(file)
            }
        }
    }

    private fun plain(key: Sym) =
        if (key.name.startsWith(FileSurface.CLASS_MARK)) Sym(key.scope, key.name.removePrefix(FileSurface.CLASS_MARK)) else key

    // "p.Box<kotlin.Int>?" -> "p.Box"
    private fun erase(type: String) = type.substringBefore('<').removeSuffix("?")

    @Synchronized
    open fun reset() {
        compileKey = null; verdictKey = null
        surfaces.clear(); contentHashes.clear(); index.clear(); changes = HashMap(); commits = 0
        classFile.clear(); declFiles.clear(); subclassFiles.clear()
        releaseSnapshot()
    }

    // ---- binary snapshot ----------------------------------------------------------------------------------

    /**
     * Takes the binary snapshot of the compile just committed, built from exactly the sources that compile read.
     * The state owns it from here and deletes it when the state is committed to again or reset. A null [snapshot]
     * records why there is none: [reason] is what a planner reports.
     */
    @Synchronized
    open fun attachSnapshot(snapshot: Snapshot?, reason: SliceFallback = SliceFallback.NO_SNAPSHOT) {
        releaseSnapshot()
        noSnapshotReason = reason
        if (snapshot == null) return
        snapshotOrNull = snapshot
        for ((file, surface) in surfaces) snapshotSurfaceHashes[file] = hashOf(surface)
    }

    /** The module's binary snapshot, or null when there is none ([fallbackReason] says why). */
    val snapshot: Snapshot? @Synchronized get() = snapshotOrNull

    /** Why a slice compile cannot be used right now, or null when the state has a usable snapshot. */
    @Synchronized fun fallbackReason(): SliceFallback? = if (snapshotOrNull != null) null else noSnapshotReason

    /** Drops the snapshot (it is corrupt, or its facts are gone): the planner falls back to a full compile. */
    @Synchronized fun dropSnapshot(reason: SliceFallback = SliceFallback.NO_SNAPSHOT) {
        releaseSnapshot()
        noSnapshotReason = reason
    }

    /** The exception of the latest failed slice compile since the last full commit, for the planner's telemetry. */
    val lastSliceFailure: Throwable? @Synchronized get() = sliceFailure

    @Synchronized fun noteSliceFailure(cause: Throwable) { sliceFailure = cause }

    /**
     * Files whose binary in the snapshot no longer describes them, canonical paths: files whose surface differs
     * from the surface their binary was built from (including files added since the snapshot), and files whose last
     * compile had a compiler error. A slice compile sees the rest of the module only through those binaries, so
     * a planner keeps every one of these in every slice until the module is re-snapshotted. A body-only edit leaves
     * a file's surface, and so its binary, valid for everything outside it: it never lands here.
     */
    @Synchronized fun binaryStale(): Set<String> = HashSet(binaryStaleFiles)

    /** Files that have a binary in the snapshot but were deleted since; every slice hides those binaries. */
    @Synchronized fun removedSinceSnapshot(): Set<String> = HashSet(removedFiles)

    /**
     * True when so many files have to be kept in every slice that a re-snapshot (one full compile with the
     * backend, a full `AnalysisSession.check`) is cheaper: at least [minFiles] binary-stale or removed files, and
     * more than [maxFraction] of the files the snapshot was built from.
     */
    @Synchronized
    fun exceedsStaleBound(maxFraction: Double = DEFAULT_STALE_FRACTION, minFiles: Int = DEFAULT_STALE_MIN_FILES): Boolean {
        val pending = binaryStaleFiles.size + removedFiles.size
        return pending >= minFiles && pending > snapshotSurfaceHashes.size * maxFraction
    }

    private fun releaseSnapshot() {
        val old = snapshotOrNull
        snapshotOrNull = null
        snapshotSurfaceHashes.clear(); binaryStaleFiles.clear(); removedFiles.clear()
        noSnapshotReason = SliceFallback.NO_SNAPSHOT
        sliceFailure = null
        try { old?.delete() } catch (_: Exception) {}
    }

    private fun hashOf(s: FileSurface) =
        sha1Hex(s.decls.entries.map { "${it.key}=${it.value}" }.sorted().joinToString("\n").toByteArray())

    companion object {
        const val DEFAULT_STALE_FRACTION = 0.10
        const val DEFAULT_STALE_MIN_FILES = 16
        private val UNCHANGED = SurfaceChange(emptySet(), emptySet(), null)
    }

    @Synchronized fun files(): Set<String> = HashSet(surfaces.keys)

    @Synchronized fun contentHash(file: String): String? = contentHashes[canonicalPath(file)]

    /** Digest of [file]'s recorded declaration surface, or null when the file is not recorded. */
    @Synchronized fun surfaceHash(file: String): String? = surfaces[canonicalPath(file)]?.let(::hashOf)

    @Synchronized fun lookupsOf(file: String): Set<Sym> = HashSet(index.lookupsOf(canonicalPath(file)))

    /** The file that declares class [classFqName] (nested classes included), or null. */
    @Synchronized fun fileDeclaring(classFqName: String): String? = classFile[classFqName]

    /** Files that declare [symbol] (a class is `Sym(scope, name)`), usually one; more than one is a redeclaration. */
    @Synchronized fun filesDeclaring(symbol: Sym): Set<String> = HashSet(declFiles[symbol].orEmpty())

    /**
     * [files] plus, for every sealed class or interface declared in them, the files of all its direct and indirect
     * subclasses (the closure over the whole sealed hierarchy). A sealed parent compiled from source only knows the
     * inheritors that are in source too, so it must be compiled with all of them: otherwise its surface (which
     * lists the inheritors) moves with the slice and exhaustiveness over it is wrong. [current] gives the surfaces
     * of files just compiled, which win over the recorded ones.
     */
    @Synchronized
    fun withSealedInheritors(files: Collection<String>, current: Map<String, FileSurface> = emptyMap()): Set<String> {
        val out = files.mapTo(LinkedHashSet()) { canonicalPath(it) }
        val queue = ArrayDeque(out)
        while (queue.isNotEmpty()) {
            val file = queue.removeFirst()
            val surface = current[file] ?: surfaces[file] ?: continue
            for (cls in surface.sealed) for (sub in subclassFiles[cls].orEmpty()) if (out.add(sub)) queue.add(sub)
        }
        return out
    }

    /** Files that declare a class with [supertypeFqName] among its supertypes (type arguments ignored). */
    @Synchronized fun subclassFilesOf(supertypeFqName: String): Set<String> = HashSet(subclassFiles[supertypeFqName].orEmpty())

    /**
     * How [file]'s surface moved in the latest commit, with its previous surface; null for a file with no
     * previous record (first commit, reset, or a new file).
     */
    @Synchronized fun changeOf(file: String): SurfaceChange? = changes[canonicalPath(file)]

    /**
     * True when [file]'s declaration surface differs from the previous commit's. A file with no previous
     * record (first commit, reset, or a new file) counts as changed; a body-only or comment edit does not.
     */
    @Synchronized fun surfaceChanged(file: String): Boolean {
        val change = changes[canonicalPath(file)] ?: return true
        return !change.isEmpty
    }

    /**
     * Files other than [changedFiles] whose last compile looked up a symbol that a changed file's
     * surface gained, lost or changed between the previous commit and the latest one (a changed class
     * header counts as its whole scope). A file with no previous record contributes everything it
     * declares now. Body-only edits change no surface and so have no dependents.
     *
     * Lookups alone do not see subclasses of a class whose member changed, nor a removed supertype; use
     * [subclassFilesOf] and [changeOf] for those.
     */
    @Synchronized fun dependentsOf(changedFiles: Collection<String>): Set<String> {
        val changed = changedFiles.mapTo(HashSet()) { canonicalPath(it) }
        val out = HashSet<String>()
        for (file in changed) {
            val change = changes[file] ?: SurfaceChange.of(null, surfaces[file])
            out += index.dependents(change.symbols, change.scopes)
        }
        out -= changed
        return out
    }
}
