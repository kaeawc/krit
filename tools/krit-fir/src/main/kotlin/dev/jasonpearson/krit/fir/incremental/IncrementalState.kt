package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.ClassNameFact
import dev.jasonpearson.krit.fir.serializableReachedNames
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
    val classFacts = ConcurrentHashMap<String, List<ClassNameFact>>()

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
     * Path, size and modification time: cheap, and changes whenever the classpath entry is rebuilt. A zip or jar file
     * also contributes [ZipDigest] of its central directory, which holds the CRC-32 of every entry, so replacing a jar
     * by one of the same size and timestamp but other contents still changes the key (a persisted bundle outlives the
     * process, and nothing else would notice). A directory (compiled classes of an upstream module) is walked and each
     * regular file's relative path, size and modification time is included, sorted; the root's own mtime does not
     * change when nested files are rewritten. File contents inside a directory are not read: build tools rewrite
     * class files with new timestamps, and hashing a whole output tree on every compile-key computation would cost
     * more than the bundle saves.
     */
    fun addFileIdentity(label: String, path: String): FingerprintBuilder {
        val f = File(path)
        if (!f.isDirectory) {
            if (!f.isFile) return add(label, listOf(canonicalPath(path), -1L, f.lastModified()))
            val size = f.length()
            val modified = f.lastModified()
            val parts = ArrayList<Any>(listOf(canonicalPath(path), size, modified))
            if (ZipDigest.looksLikeZip(path)) parts += ZipDigest.of(f, size, modified)
            return add(label, parts)
        }
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
 * A digest of a zip or jar file's central directory: every entry's name, sizes, CRC-32 and timestamps, in a few
 * kilobytes to megabytes read from the end of the file, instead of the whole archive. A rebuilt archive with other
 * contents has other CRCs. Cached per path, size and modification time, so a classpath is read once per process.
 * Anything the reader does not understand digests to a fixed marker (the size and mtime still count).
 */
internal object ZipDigest {
    private const val EOCD_SIGNATURE = 0x06054b50
    private const val ZIP64_LOCATOR_SIGNATURE = 0x07064b50
    private const val ZIP64_EOCD_SIGNATURE = 0x06064b50
    private const val EOCD_MIN = 22
    private const val MAX_COMMENT = 0xFFFF
    private const val MAX_CACHE = 20_000

    private data class Key(val path: String, val size: Long, val modified: Long)

    // Keyed by path, size and mtime, so inside one live process a jar replaced by one of the same size and mtime keeps
    // its old digest (the bundle this guards is read by the next process, which starts with an empty cache). When the
    // cache reaches MAX_CACHE entries it is cleared whole and refilled by the entries that are used next.

    private val cache = ConcurrentHashMap<Key, String>()

    /** Forgets what was read, as a new process would. For tests. */
    internal fun clearCache() = cache.clear()

    fun looksLikeZip(path: String): Boolean {
        val name = path.lowercase()
        return name.endsWith(".jar") || name.endsWith(".zip") || name.endsWith(".aar") || name.endsWith(".klib")
    }

    fun of(file: File, size: Long = file.length(), modified: Long = file.lastModified()): String {
        val key = Key(canonicalPath(file.path), size, modified)
        cache[key]?.let { return it }
        val digest = try { compute(file, size) } catch (_: Exception) { "unreadable" }
        if (cache.size >= MAX_CACHE) cache.clear()
        cache[key] = digest
        return digest
    }

    private const val CD_SIGNATURE = 0x02014b50

    private fun compute(file: File, size: Long): String = java.io.RandomAccessFile(file, "r").use { raf ->
        if (size < EOCD_MIN) return@use "not-a-zip"
        val tailLength = minOf(size, (EOCD_MIN + MAX_COMMENT).toLong()).toInt()
        val tailStart = size - tailLength
        val tail = ByteArray(tailLength)
        raf.seek(tailStart)
        raf.readFully(tail)
        val buf = java.nio.ByteBuffer.wrap(tail).order(java.nio.ByteOrder.LITTLE_ENDIAN)
        var sawCandidate = false
        var emptyDirectory = false
        // End-record candidates, last first. A candidate counts when its comment reaches exactly to the end of the file
        // (as in java.util.zip) and the bytes where its central directory should start are a central directory header:
        // a lookalike at the end of the comment fails that, and the search goes on to the real one.
        for (i in tailLength - EOCD_MIN downTo 0) {
            if (buf.getInt(i) != EOCD_SIGNATURE || i + EOCD_MIN + (buf.getShort(i + 20).toInt() and 0xFFFF) != tailLength) continue
            sawCandidate = true
            var cdSize = buf.getInt(i + 12).toLong() and 0xFFFFFFFFL
            var entries = (buf.getShort(i + 10).toInt() and 0xFFFF).toLong()
            // The directory ends where its end record begins. Offsets recorded in the archive are relative to the start
            // of the archive, which is not the start of the file when data is prepended (launcher-script jars).
            var cdEnd = tailStart + i
            if (cdSize == 0xFFFFFFFFL || (buf.getInt(i + 16).toLong() and 0xFFFFFFFFL) == 0xFFFFFFFFL || entries == 0xFFFFL) {
                val rec = zip64Record(raf, buf, tailStart, i) ?: continue
                cdEnd = rec.at; cdSize = rec.cdSize; entries = rec.entries
            }
            if (cdSize == 0L && entries == 0L) { emptyDirectory = true; continue }
            val cdStart = cdEnd - cdSize
            if (cdSize < 0 || cdStart < 0 || cdEnd > size) continue
            val head = ByteArray(4)
            raf.seek(cdStart)
            raf.readFully(head)
            if (java.nio.ByteBuffer.wrap(head).order(java.nio.ByteOrder.LITTLE_ENDIAN).getInt(0) != CD_SIGNATURE) continue
            return@use digestOf(raf, cdStart, cdSize) ?: "short-directory"
        }
        // An archive with no entries has an empty directory, which is a fine thing to digest if nothing else qualified.
        if (emptyDirectory) return@use digestOf(raf, 0, 0) ?: "short-directory"
        if (sawCandidate) "bad-directory" else "not-a-zip"
    }

    private class Zip64Record(val at: Long, val cdSize: Long, val entries: Long)

    // The zip64 end record that goes with the plain end record at [eocd] of the tail: it ends where the locator, which
    // sits just before the plain record, begins. It is 56 bytes unless it carries an extensible data sector, so it is
    // found by its signature and a size field that reaches exactly to the locator, and it cannot start before the
    // offset the locator records (the archive may be preceded by data, never the other way round).
    private fun zip64Record(raf: java.io.RandomAccessFile, buf: java.nio.ByteBuffer, tailStart: Long, eocd: Int): Zip64Record? {
        val locator = eocd - 20
        if (locator < 0 || buf.getInt(locator) != ZIP64_LOCATOR_SIGNATURE) return null
        val locatorAt = tailStart + locator
        val recorded = buf.getLong(locator + 8)
        val from = maxOf(0L, locatorAt - 56 - 65_536)
        val window = ByteArray((locatorAt - from).toInt())
        if (window.size < 56) return null
        raf.seek(from)
        raf.readFully(window)
        val w = java.nio.ByteBuffer.wrap(window).order(java.nio.ByteOrder.LITTLE_ENDIAN)
        for (q in window.size - 56 downTo 0) {
            if (w.getInt(q) != ZIP64_EOCD_SIGNATURE) continue
            val declared = w.getLong(q + 4)
            if (declared < 44 || from + q + 12 + declared != locatorAt || from + q < recorded) continue
            return Zip64Record(from + q, w.getLong(q + 40), w.getLong(q + 32))
        }
        return null
    }

    private fun digestOf(raf: java.io.RandomAccessFile, start: Long, length: Long): String? {
        val sha = MessageDigest.getInstance("SHA-256")
        sha.update(length.toString().toByteArray())
        raf.seek(start)
        val chunk = ByteArray(64 * 1024)
        var left = length
        while (left > 0) {
            val n = raf.read(chunk, 0, minOf(left, chunk.size.toLong()).toInt())
            if (n < 0) return null
            sha.update(chunk, 0, n)
            left -= n
        }
        return hex(sha.digest())
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
 * declaration-surface hashes, and the reverse index of what each file's last compile looked up. The
 * slice-compile planner (`IncrementalPlanner`) reads it.
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
    private val classFacts = HashMap<String, List<ClassNameFact>>()
    private var reachedNames: Set<String>? = null // derived from classFacts, computed on demand
    private val index = LookupIndex()
    private var changes = HashMap<String, SurfaceChange>()
    private var commits = 0

    // Declaration indexes over the latest commit, for the structural rules lookups cannot see.
    private val classFile = HashMap<String, String>()
    private val declFiles = HashMap<Sym, MutableSet<String>>()
    private val subclassFiles = HashMap<String, MutableSet<String>>()
    private val packageFiles = HashMap<String, MutableSet<String>>() // package -> files that declare it
    private val subclassNames = HashMap<String, MutableSet<String>>() // erased supertype -> fq names of classes extending it

    // The binary snapshot of the module and what became of it since it was built.
    private var snapshotOrNull: Snapshot? = null
    private var noSnapshotReason = SliceFallback.NO_SNAPSHOT
    private var noSnapshotDetail: String? = null
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
        classFacts.clear(); classFacts.putAll(recording.classFacts); reachedNames = null
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

        var factsChanged = false
        for (file in removedSet) {
            surfaces.remove(file); contentHashes.remove(file); index.remove(file)
            if (classFacts.remove(file) != null) factsChanged = true
            binaryStaleFiles.remove(file)
            if (file in snapshotSurfaceHashes) removedFiles += file
        }
        for (file in compiledSet) {
            recording.surfaces[file]?.let { surfaces[file] = it } ?: surfaces.remove(file)
            recording.contentHashes[file]?.let { contentHashes[file] = it } ?: contentHashes.remove(file)
            val facts = recording.classFacts[file]
            if (facts != classFacts[file]) factsChanged = true
            if (facts != null) classFacts[file] = facts else classFacts.remove(file)
            index.update(file, recording.lookups.byFile[file] ?: emptySet())
            removedFiles.remove(file)
            if (snapshotOrNull != null) {
                val builtFrom = snapshotSurfaceHashes[file]
                val now = surfaces[file]?.let(::hashOf)
                if (file in errored || builtFrom == null || now != builtFrom) binaryStaleFiles += file else binaryStaleFiles -= file
            }
        }
        // The reached-name set is a function of the facts alone: keep it when no compiled file's facts moved.
        if (factsChanged) reachedNames = null
        rebuildDeclarationIndexes()
        commits++
    }

    private fun rebuildDeclarationIndexes() {
        classFile.clear(); declFiles.clear(); subclassFiles.clear(); subclassNames.clear(); packageFiles.clear()
        for ((file, surface) in surfaces) {
            packageFiles.getOrPut(surface.pkg) { HashSet() }.add(file)
            for (key in surface.decls.keys) declFiles.getOrPut(plain(key)) { HashSet() }.add(file)
            for ((cls, supers) in surface.supers) {
                classFile[cls] = file
                for (s in supers) {
                    subclassFiles.getOrPut(erase(s)) { HashSet() }.add(file)
                    subclassNames.getOrPut(erase(s)) { HashSet() }.add(cls)
                }
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
        surfaces.clear(); contentHashes.clear(); classFacts.clear(); reachedNames = null; index.clear(); changes = HashMap(); commits = 0
        classFile.clear(); declFiles.clear(); subclassFiles.clear(); subclassNames.clear(); packageFiles.clear()
        releaseSnapshot()
    }

    // ---- persistence --------------------------------------------------------------------------------------

    /**
     * The persistable image of the state (see [BundleCodec]), or null when there is nothing worth keeping: no commit
     * yet, or no usable snapshot (a state without one cannot serve a slice, so a bundle of it would only be a cold
     * start with extra steps). The maps are copied; their values are immutable.
     */
    @Synchronized
    fun exportImage(): PersistedState? {
        val key = compileKey ?: return null
        val snap = snapshotOrNull ?: return null
        if (snap.corrupt || commits == 0) return null
        val lookups = HashMap<String, Set<Sym>>(index.files.size * 2)
        for (file in index.files) lookups[file] = index.lookupsOf(file)
        return PersistedState(
            key, verdictKey, commits, HashMap(surfaces), HashMap(contentHashes), HashMap(classFacts), lookups,
            PersistedSnapshot(snap.outputsCopy(), HashMap(snapshotSurfaceHashes), HashSet(binaryStaleFiles), HashSet(removedFiles)),
        )
    }

    /**
     * Replaces the state with a persisted image and the [snapshot] whose class files it describes (already restored,
     * see [Snapshot.restore]); the declaration indexes are rebuilt from the surfaces. Returns false, leaving the state
     * reset, when the image is not one a commit could have produced. The surface changes of the latest commit are not part of an image: a
     * loaded state reports none, like a state nothing has been committed to since.
     */
    @Synchronized
    fun restoreImage(image: PersistedState, snapshot: Snapshot): Boolean {
        reset()
        if (image.generation < 1) return false
        compileKey = image.compileKey
        verdictKey = image.verdictKey
        surfaces.putAll(image.surfaces)
        contentHashes.putAll(image.contentHashes)
        classFacts.putAll(image.classFacts)
        for ((file, syms) in image.lookups) index.update(file, syms)
        rebuildDeclarationIndexes()
        commits = image.generation
        snapshotOrNull = snapshot
        snapshotSurfaceHashes.putAll(image.snapshot.surfaceHashes)
        binaryStaleFiles.addAll(image.snapshot.binaryStale)
        removedFiles.addAll(image.snapshot.removed)
        return true
    }

    // ---- binary snapshot ----------------------------------------------------------------------------------

    /**
     * Takes the binary snapshot of the compile just committed, built from exactly the sources that compile read.
     * The state owns it from here and deletes it when the state is committed to again or reset. A null [snapshot]
     * records why there is none: [reason] is what a planner reports.
     */
    @Synchronized
    open fun attachSnapshot(snapshot: Snapshot?, reason: SliceFallback = SliceFallback.NO_SNAPSHOT, detail: String? = null) {
        releaseSnapshot()
        noSnapshotReason = reason
        noSnapshotDetail = detail
        if (snapshot == null) return
        snapshotOrNull = snapshot
        for ((file, surface) in surfaces) snapshotSurfaceHashes[file] = hashOf(surface)
    }

    /** The module's binary snapshot, or null when there is none ([fallbackReason] says why). */
    val snapshot: Snapshot? @Synchronized get() = snapshotOrNull

    /** Why a slice compile cannot be used right now, or null when the state has a usable snapshot. */
    @Synchronized fun fallbackReason(): SliceFallback? = if (snapshotOrNull != null) null else noSnapshotReason

    /** More about [fallbackReason] when there is more to say (for [SliceFallback.COMPILE_ERRORS], how many files). */
    @Synchronized fun fallbackDetail(): String? = if (snapshotOrNull != null) null else noSnapshotDetail

    /** Drops the snapshot (it is corrupt, or its facts are gone): the planner falls back to a full compile. */
    @Synchronized fun dropSnapshot(reason: SliceFallback = SliceFallback.NO_SNAPSHOT) {
        releaseSnapshot()
        noSnapshotReason = reason
        noSnapshotDetail = null
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
        noSnapshotDetail = null
        sliceFailure = null
        try { old?.delete() } catch (_: Exception) {}
    }

    private fun hashOf(s: FileSurface) =
        sha1Hex(s.decls.entries.map { "${it.key}=${it.value}" }.sorted().joinToString("\n").toByteArray())

    companion object {
        const val DEFAULT_STALE_FRACTION = 0.05
        const val DEFAULT_STALE_MIN_FILES = 16
        private val UNCHANGED = SurfaceChange(emptySet(), emptySet(), null)
    }

    /**
     * The recorded class-name facts of every file except [excluding] (canonical paths), for a slice to hand to
     * rules as the facts of the module files it does not compile from source.
     */
    @Synchronized
    fun classFactsExcluding(excluding: Collection<String>): List<ClassNameFact> {
        val skip = excluding.toHashSet()
        val out = ArrayList<ClassNameFact>()
        for ((file, facts) in classFacts) if (file !in skip) out += facts
        return out
    }

    /**
     * The class names that reach `Serializable` by supertype names, over the recorded facts of the whole module
     * (see [serializableReachedNames]). Derived from the facts, so it follows every commit and slice merge.
     */
    @Synchronized
    fun serializableNames(): Set<String> = reachedNames ?: serializableReachedNames(classFacts.values.asSequence().flatten().asIterable()).also { reachedNames = it }

    @Synchronized fun files(): Set<String> = HashSet(surfaces.keys)

    @Synchronized fun contentHash(file: String): String? = contentHashes[canonicalPath(file)]

    /** The content hash of every recorded file, keyed by canonical path. */
    @Synchronized fun contentHashes(): Map<String, String> = HashMap(contentHashes)

    /** Digest of [file]'s recorded declaration surface, or null when the file is not recorded. */
    @Synchronized fun surfaceHash(file: String): String? = surfaces[canonicalPath(file)]?.let(::hashOf)

    @Synchronized fun lookupsOf(file: String): Set<Sym> = HashSet(index.lookupsOf(canonicalPath(file)))

    /** The file that declares class [classFqName] (nested classes included), or null. */
    @Synchronized fun fileDeclaring(classFqName: String): String? = classFile[classFqName]

    /** Files that declare [symbol] (a class is `Sym(scope, name)`), usually one; more than one is a redeclaration. */
    @Synchronized fun filesDeclaring(symbol: Sym): Set<String> = HashSet(declFiles[symbol].orEmpty())

    /**
     * [files] plus the whole sealed hierarchy of every sealed class or interface they touch, in both directions:
     *  - for a sealed class or interface declared in a file, the files of all its direct and indirect subclasses;
     *  - for a class a file declares (or declared: [files] may name deleted files, whose recorded surface counts) with
     *    a direct supertype that is a sealed class or interface of the module, that parent's file, and through it the
     *    parent's other inheritors. The supertypes of both the recorded surface and the one in [current] are
     *    looked at, so a subclass that is added, removed or retargeted to another parent brings the parent in.
     *
     * A sealed parent compiled from source knows only the inheritors that are in source too, so it must be compiled
     * with all of them; otherwise its inheritor list (part of its surface) and every exhaustiveness check over it is
     * wrong. [current] gives the surfaces of files just compiled, which win over the recorded ones for the sealed
     * classes they declare.
     */
    @Synchronized
    fun withSealedInheritors(files: Collection<String>, current: Map<String, FileSurface> = emptyMap()): Set<String> {
        val out = files.mapTo(LinkedHashSet()) { canonicalPath(it) }
        val queue = ArrayDeque(out)
        fun add(file: String) { if (out.add(file)) queue.add(file) }
        fun isSealed(cls: String): Boolean {
            val declaring = classFile[cls] ?: return false
            return cls in (current[declaring] ?: surfaces[declaring])?.sealed.orEmpty()
        }
        while (queue.isNotEmpty()) {
            val file = queue.removeFirst()
            val recorded = surfaces[file]
            val now = current[file]
            for (surface in listOfNotNull(recorded, now)) {
                for ((_, supers) in surface.supers) for (super_ in supers) {
                    val parent = erase(super_)
                    if (isSealed(parent)) classFile[parent]?.let(::add)
                }
            }
            for (cls in (now ?: recorded)?.sealed.orEmpty() + recorded?.sealed.orEmpty()) {
                for (sub in subclassFiles[cls].orEmpty()) add(sub)
            }
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

    /** True when some recorded file declares [pkg] ("" is the root package). */
    @Synchronized fun packageDeclared(pkg: String): Boolean = packageFiles[pkg].orEmpty().isNotEmpty()

    /** The recorded declaration surface of [file], or null when the file is not recorded. */
    @Synchronized fun surfaceOf(file: String): FileSurface? = surfaces[canonicalPath(file)]

    /** Fully qualified names of the classes that extend [classFqName], directly or through other classes. */
    private fun subclassesTransitive(classFqName: String): Set<String> {
        val seen = LinkedHashSet<String>()
        val stack = ArrayDeque(listOf(classFqName))
        while (stack.isNotEmpty()) for (sub in subclassNames[stack.removeLast()].orEmpty()) if (seen.add(sub)) stack += sub
        return seen
    }

    /**
     * [dependentsOf] for a planner, with one more rule: a member that changed on a class also changed on every
     * subclass, because a call through a subclass looks the name up in the subclass's scope and never in the
     * class that declares it.
     */
    @Synchronized fun dependentsWithSubclassScopes(changedFiles: Collection<String>): Set<String> {
        val changed = changedFiles.mapTo(HashSet()) { canonicalPath(it) }
        val out = HashSet<String>()
        for (file in changed) {
            val change = changes[file] ?: SurfaceChange.of(null, surfaces[file])
            val symbols = HashSet(change.symbols)
            val scopes = HashSet(change.scopes)
            for (sym in change.symbols) {
                if (sym.scope.isEmpty()) continue
                for (sub in subclassesTransitive(sym.scope)) symbols += Sym(sub, sym.name)
            }
            for (scope in change.scopes) scopes += subclassesTransitive(scope)
            out += index.dependents(symbols, scopes)
        }
        out -= changed
        return out
    }

    /**
     * The files a surface change reaches that lookups cannot see, for the files of the latest commit
     * ([changeOf] covers that commit only). For each declaration that was added, removed or changed:
     *  - a class: the declaring files of its supertypes, before and after (a sealed parent's binary lists its
     *    inheritors);
     *  - a top-level declaration: every other file that declares the same name (a redeclaration or conflicting
     *    overload is only seen with both sides in source, and disappears when one side goes);
     *  - a member (or a class): the files of all subclasses of the class that owns it (an implementer of an
     *    abstract member that was renamed never looks the new name up).
     * [files] are excluded from the result.
     */
    @Synchronized fun structuralDependentsOf(files: Collection<String>): Set<String> {
        val given = files.mapTo(HashSet()) { canonicalPath(it) }
        val out = HashSet<String>()
        for (file in given) {
            val change = changes[file] ?: SurfaceChange.of(null, surfaces[file])
            if (change.isEmpty) continue
            val before = change.previous
            val after = surfaces[file]
            val old = before?.decls.orEmpty()
            val new = after?.decls.orEmpty()
            for (key in old.keys + new.keys) {
                if (old[key] == new[key]) continue
                val isClass = key.name.startsWith(FileSurface.CLASS_MARK)
                val fq = if (isClass) (if (key.scope.isEmpty()) "" else key.scope + ".") + key.name.removePrefix(FileSurface.CLASS_MARK) else null
                if (fq != null) {
                    for (super_ in before?.supers?.get(fq).orEmpty() + after?.supers?.get(fq).orEmpty()) classFile[erase(super_)]?.let(out::add)
                }
                if (classFile[key.scope] == null) declFiles[plain(key)]?.let(out::addAll)
                val owner = fq ?: key.scope
                if (classFile[owner] != null) for (sub in subclassesTransitive(owner)) classFile[sub]?.let(out::add)
            }
        }
        out -= given
        return out
    }
}
