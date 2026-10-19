package dev.jasonpearson.krit.fir.incremental

import java.io.File
import java.security.MessageDigest
import java.util.concurrent.ConcurrentHashMap

/** What one full compile recorded: lookups and declaration surfaces per canonical source path, plus content hashes. */
internal class CompileRecording(val fingerprint: String, val contentHashes: Map<String, String>) {
    val lookups = CompileLookups()
    val surfaces = ConcurrentHashMap<String, FileSurface>()
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

internal fun sha1Hex(bytes: ByteArray): String =
    MessageDigest.getInstance("SHA-1").digest(bytes).joinToString("") { "%02x".format(it) }

/** Content hash of each source file, keyed by canonical path; unreadable files are left out. */
internal fun contentHashesOf(paths: Collection<String>): Map<String, String> {
    val out = LinkedHashMap<String, String>()
    for (path in paths) {
        try { out[canonicalPath(path)] = sha1Hex(File(path).readBytes()) } catch (_: Exception) {}
    }
    return out
}

/**
 * Digest of everything that makes one compile's recorded facts incomparable with another's: session
 * settings, classpath identity, compiler and plugin identity, enabled rules and their configs, test-file
 * classification, scan paths and SDK levels. Add the parts in a fixed order.
 */
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

    fun build(): String = digest.digest().joinToString("") { "%02x".format(it) }

    private fun render(value: Any?): String = when (value) {
        null -> "null"
        is Map<*, *> -> value.entries.map { render(it.key) to it.value }.sortedBy { it.first }
            .joinToString(",", "{", "}") { "${it.first}=${render(it.second)}" }
        is Set<*> -> value.map { render(it) }.sorted().joinToString(",", "[", "]")
        is Iterable<*> -> value.joinToString(",", "(", ")") { render(it) }
        else -> value.toString()
    }
}

/** How a file's declaration surface moved between the two latest recorded compiles. */
internal class SurfaceChange(val symbols: Set<Sym>, val scopes: Set<String>) {
    val isEmpty: Boolean get() = symbols.isEmpty() && scopes.isEmpty()

    companion object {
        fun of(diff: Pair<Set<Sym>, Set<String>>) = SurfaceChange(diff.first, diff.second)
    }
}

/**
 * Incremental facts of one [dev.jasonpearson.krit.fir.runner.AnalysisSession]: per-file content hashes and
 * declaration-surface hashes, and the reverse index of what each file's last compile looked up. Nothing
 * consumes it yet; the slice-compile planner will.
 *
 * Every [commit] is a whole-module compile and replaces the per-file facts. A commit under a different
 * invocation fingerprint resets the state first, so facts recorded under other classpath, rules or options
 * are never compared with new ones.
 */
internal class IncrementalState {
    var fingerprint: String? = null
        private set

    private val surfaces = HashMap<String, FileSurface>()
    private val contentHashes = HashMap<String, String>()
    private val index = LookupIndex()
    private var changes = HashMap<String, SurfaceChange>()
    private var commits = 0

    /** Number of commits since the last reset. */
    val generation: Int @Synchronized get() = commits

    @Synchronized
    fun commit(recording: CompileRecording) {
        val previous = if (fingerprint == recording.fingerprint) HashMap(surfaces) else null
        if (previous == null) reset()
        fingerprint = recording.fingerprint
        val next = HashMap<String, SurfaceChange>()
        if (previous != null) {
            for (file in previous.keys + recording.surfaces.keys) {
                next[file] = SurfaceChange.of(FileSurface.diff(previous[file], recording.surfaces[file]))
            }
        }
        changes = next
        surfaces.clear(); surfaces.putAll(recording.surfaces)
        contentHashes.clear(); contentHashes.putAll(recording.contentHashes)
        for (file in index.files.toList()) if (!recording.surfaces.containsKey(file)) index.remove(file)
        for ((file, syms) in recording.lookups.byFile) index.update(file, syms)
        commits++
    }

    @Synchronized
    fun reset() {
        fingerprint = null
        surfaces.clear(); contentHashes.clear(); index.clear(); changes = HashMap(); commits = 0
    }

    @Synchronized fun files(): Set<String> = HashSet(surfaces.keys)

    @Synchronized fun contentHash(file: String): String? = contentHashes[canonicalPath(file)]

    /** Digest of [file]'s recorded declaration surface, or null when the file is not recorded. */
    @Synchronized fun surfaceHash(file: String): String? = surfaces[canonicalPath(file)]?.let { s ->
        sha1Hex(s.decls.entries.map { "${it.key}=${it.value}" }.sorted().joinToString("\n").toByteArray())
    }

    @Synchronized fun lookupsOf(file: String): Set<Sym> = HashSet(index.lookupsOf(canonicalPath(file)))

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
     */
    @Synchronized fun dependentsOf(changedFiles: Collection<String>): Set<String> {
        val changed = changedFiles.mapTo(HashSet()) { canonicalPath(it) }
        val out = HashSet<String>()
        for (file in changed) {
            val change = changes[file] ?: SurfaceChange.of(FileSurface.diff(null, surfaces[file]))
            out += index.dependents(change.symbols, change.scopes)
        }
        out -= changed
        return out
    }
}
