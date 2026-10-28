package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.oracle.AnalyzeResult
import dev.jasonpearson.krit.fir.oracle.ClassPayload
import dev.jasonpearson.krit.fir.oracle.FilePayload
import dev.jasonpearson.krit.fir.oracle.OracleResponse
import dev.jasonpearson.krit.fir.oracle.mergeDeclared

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
    /**
     * [perFileDeps] names, per supertype FQN, the module file declaring it: the projection is taken from that file's
     * entry when an answer is assembled, so an edit to the supertype that leaves this file alone (a private member,
     * which no dependent's surface sees) still shows. [declaredAnnotations] are the source-side annotations of the
     * file's annotated functions and constructors, by `calleeSignature`.
     */
    class Entry(
        val contentHash: String,
        val payload: FilePayload,
        val depPaths: List<String> = emptyList(),
        val perFileDeps: Map<String, String> = emptyMap(),
        val declaredAnnotations: Map<String, List<String>> = emptyMap(),
    )

    /** An oracle answer assembled from the cache, in the shapes a compile returns. */
    class Assembled(val result: AnalyzeResult, val cacheDeps: OracleResponse.CacheDepsView)

    private val entries = HashMap<String, Entry>()

    // The file order of the last whole-module oracle compile, which an assembled answer keeps, so it reads like one.
    private val order = LinkedHashSet<String>()

    /** The [IncrementalState.generation] this cache was last brought up to; -1 when it holds nothing usable. */
    var generation: Int = -1
        private set

    val size: Int get() = entries.size

    fun get(file: String): Entry? = entries[file]

    fun put(file: String, entry: Entry) { entries[file] = entry }

    fun remove(file: String) { entries.remove(file); order.remove(file) }

    fun clear() { entries.clear(); order.clear(); generation = -1 }

    /** Records the file order (canonical paths) of a whole-module oracle compile. */
    fun recordOrder(files: Collection<String>) { order.clear(); order.addAll(files) }

    fun sync(stateGeneration: Int) { generation = stateGeneration }

    /** An immutable image of the cache for the bundle (#740); entries are immutable, so sharing them is safe. */
    fun export(): PersistedOracle = PersistedOracle(generation, order.filter { it in entries }, LinkedHashMap(entries))

    /** Replaces the cache with a persisted image, as of state generation [stateGeneration]. */
    fun restore(image: PersistedOracle, stateGeneration: Int) {
        clear()
        entries.putAll(image.entries)
        order.addAll(image.order)
        generation = stateGeneration
    }

    /**
     * The source-side annotations of every annotated callable the module declares outside [excluding] (canonical
     * paths): what a slice that reads those files as class files reports for calls to them.
     */
    fun declaredAnnotationIndex(excluding: Set<String>): Map<String, List<String>> {
        val out = HashMap<String, List<String>>()
        for ((file, entry) in entries) {
            if (file in excluding) continue
            for ((signature, annotations) in entry.declaredAnnotations) out[signature] = mergeDeclared(out[signature], annotations)
        }
        return out
    }

    /** The recorded projection of class [fqn] declared in [file], or null. */
    fun declaration(file: String, fqn: String): ClassPayload? =
        entries[file]?.payload?.declarations?.firstOrNull { it.fqn == fqn }

    /**
     * The oracle answer for [files] (canonical path to the spelling the response uses) from
     * the cached entries; null when one has no entry for its current content [hashes]. `dependencies` is rebuilt as
     * a compile builds it: every declared class by FQN, the first declaration winning. Files come in the order of
     * the last whole-module compile, new files after them in the order of [files].
     */
    fun assemble(files: Map<String, String>, hashes: Map<String, String>): Assembled? {
        val out = LinkedHashMap<String, FilePayload>(files.size)
        val dependencies = LinkedHashMap<String, ClassPayload>()
        val depPaths = LinkedHashMap<String, Set<String>>()
        val perFileDeps = LinkedHashMap<String, Map<String, ClassPayload>>()
        val ordered = order.filter { it in files } + files.keys.filter { it !in order }
        for (file in ordered) {
            val spelling = files.getValue(file)
            val entry = entries[file] ?: return null
            if (entry.contentHash != hashes[file]) return null
            out[spelling] = entry.payload
            for (declaration in entry.payload.declarations) dependencies.putIfAbsent(declaration.fqn, declaration)
            if (entry.depPaths.isNotEmpty()) depPaths[spelling] = entry.depPaths.mapTo(LinkedHashSet()) { files[it] ?: it }
            if (entry.perFileDeps.isNotEmpty()) {
                val projections = LinkedHashMap<String, ClassPayload>()
                for ((fqn, declaring) in entry.perFileDeps) projections[fqn] = declaration(declaring, fqn) ?: return null
                perFileDeps[spelling] = projections
            }
        }
        return Assembled(
            AnalyzeResult(files = out, dependencies = dependencies),
            OracleResponse.CacheDepsView(depPaths, perFileDeps, emptyMap()),
        )
    }
}
