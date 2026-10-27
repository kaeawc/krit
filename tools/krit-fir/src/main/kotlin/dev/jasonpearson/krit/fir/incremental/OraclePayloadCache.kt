package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.oracle.AnalyzeResult
import dev.jasonpearson.krit.fir.oracle.ClassPayload
import dev.jasonpearson.krit.fir.oracle.FilePayload
import dev.jasonpearson.krit.fir.oracle.OracleResponse

/**
 * The per-file oracle facts of an [IncrementalState]'s module, so a warm `analyzeWithCheck` can answer the oracle
 * from the planner's slice compiles instead of compiling the whole module again (#740).
 *
 * A file's facts depend on the same things its check verdict does: its own content and the surfaces of what it
 * resolves against. So an entry follows the [VerdictCache] rule: it is valid while the file's content still hashes
 * to [Entry.contentHash] and no planner wave reached the file since, and a wave that reaches it compiles it and
 * replaces the entry. Like the verdicts, the cache is touched only under the session's snapshot lock and together
 * with the state, and [generation] detects a cache that missed a commit.
 *
 * Each entry also keeps the file's `cacheDeps` edges (the module files declaring its supertypes, and their
 * projections), which Go's oracle cache invalidates on. Paths are canonical; [assemble] spells them as the request.
 */
internal class OraclePayloadCache {
    class Entry(
        val contentHash: String,
        val payload: FilePayload,
        val depPaths: List<String> = emptyList(),
        val perFileDeps: Map<String, ClassPayload> = emptyMap(),
    )

    /** An oracle answer assembled from the cache, in the shapes a compile returns. */
    class Assembled(val result: AnalyzeResult, val cacheDeps: OracleResponse.CacheDepsView)

    private val entries = HashMap<String, Entry>()

    /** The [IncrementalState.generation] this cache was last brought up to; -1 when it holds nothing usable. */
    var generation: Int = -1
        private set

    val size: Int get() = entries.size

    fun get(file: String): Entry? = entries[file]

    fun put(file: String, entry: Entry) { entries[file] = entry }

    fun remove(file: String) { entries.remove(file) }

    fun clear() { entries.clear(); generation = -1 }

    fun sync(stateGeneration: Int) { generation = stateGeneration }

    /** The recorded projection of class [fqn] declared in [file], or null. */
    fun declaration(file: String, fqn: String): ClassPayload? =
        entries[file]?.payload?.declarations?.firstOrNull { it.fqn == fqn }

    /**
     * The oracle answer for [files] (canonical path to the spelling the response uses, in compilation order) from
     * the cached entries; null when one has no entry for its current content [hashes]. `dependencies` is rebuilt as
     * a compile builds it: every declared class by FQN, the first declaration winning.
     */
    fun assemble(files: Map<String, String>, hashes: Map<String, String>): Assembled? {
        val out = LinkedHashMap<String, FilePayload>(files.size)
        val dependencies = LinkedHashMap<String, ClassPayload>()
        val depPaths = LinkedHashMap<String, Set<String>>()
        val perFileDeps = LinkedHashMap<String, Map<String, ClassPayload>>()
        for ((file, spelling) in files) {
            val entry = entries[file] ?: return null
            if (entry.contentHash != hashes[file]) return null
            out[spelling] = entry.payload
            for (declaration in entry.payload.declarations) dependencies.putIfAbsent(declaration.fqn, declaration)
            if (entry.depPaths.isNotEmpty()) depPaths[spelling] = entry.depPaths.mapTo(LinkedHashSet()) { files[it] ?: it }
            if (entry.perFileDeps.isNotEmpty()) perFileDeps[spelling] = entry.perFileDeps
        }
        return Assembled(
            AnalyzeResult(files = out, dependencies = dependencies),
            OracleResponse.CacheDepsView(depPaths, perFileDeps, emptyMap()),
        )
    }
}
