package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.security.MessageDigest
import java.util.HexFormat
import java.util.concurrent.ConcurrentHashMap

/**
 * What one whole-module compile recorded: lookups and declaration surfaces per canonical source path, plus
 * content hashes. [compileKey] identifies the compiler facts (lookups, surfaces); [verdictKey] identifies what
 * a verdict additionally depends on (rules, configs, requested files).
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

    /** Path, size and modification time: cheap, and changes whenever the classpath entry is rebuilt. */
    fun addFileIdentity(label: String, path: String): FingerprintBuilder {
        val f = File(path)
        return add(label, listOf(canonicalPath(path), if (f.isFile) f.length() else -1L, f.lastModified()))
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
 * [verdictKey] (rules, configs, requested files) only records which verdict inputs the latest commit used; a
 * verdict cache keyed by it can sit on top without losing lookups or surfaces when the rules change.
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

    /** Number of commits since the last reset. */
    val generation: Int @Synchronized get() = commits

    @Synchronized
    open fun commit(recording: CompileRecording) {
        val previous = if (compileKey == recording.compileKey) HashMap(surfaces) else null
        if (previous == null) reset()
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
    }

    @Synchronized fun files(): Set<String> = HashSet(surfaces.keys)

    @Synchronized fun contentHash(file: String): String? = contentHashes[canonicalPath(file)]

    /** Digest of [file]'s recorded declaration surface, or null when the file is not recorded. */
    @Synchronized fun surfaceHash(file: String): String? = surfaces[canonicalPath(file)]?.let { s ->
        sha1Hex(s.decls.entries.map { "${it.key}=${it.value}" }.sorted().joinToString("\n").toByteArray())
    }

    @Synchronized fun lookupsOf(file: String): Set<Sym> = HashSet(index.lookupsOf(canonicalPath(file)))

    /** The file that declares class [classFqName] (nested classes included), or null. */
    @Synchronized fun fileDeclaring(classFqName: String): String? = classFile[classFqName]

    /** Files that declare [symbol] (a class is `Sym(scope, name)`), usually one; more than one is a redeclaration. */
    @Synchronized fun filesDeclaring(symbol: Sym): Set<String> = HashSet(declFiles[symbol].orEmpty())

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
