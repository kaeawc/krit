package dev.jasonpearson.krit.fir.oracle

/**
 * Per-analyzed-file dependency-closure tracker. Feeds the Go-side
 * cache layer (which reads `cacheDeps` from the `analyzeWithDeps`
 * envelope) so it can populate its per-file dependency index.
 *
 * For each analyzed file we track:
 *
 * - `depPathsByFile` — the set of OTHER source files whose declarations
 *   the analyzed file touched (e.g. supertypes resolved against a
 *   class defined in another file in the same compilation). The
 *   Go-side cache uses this to invalidate this file's cache entry
 *   when one of those source files changes.
 * - `perFileDeps` — per-file `FQN → ClassPayload` map for the
 *   dependency classes observed while analyzing this file. Each cache
 *   entry is self-contained: it carries the dependency-class
 *   projections it needs so a single-file cache hit doesn't have to
 *   pull from the global `dependencies` map of a different run.
 * - `crashedFiles` — files that deterministically failed analysis.
 *   The K2 plugin compiles in a single batch so crashes are rare;
 *   the map exists so the wire shape stays complete and ready when
 *   per-file recovery lands.
 */
internal class DepTracker {

    private val depPaths = LinkedHashMap<String, LinkedHashSet<String>>()
    private val depsByFile = LinkedHashMap<String, LinkedHashMap<String, ClassPayload>>()
    private val crashes = LinkedHashMap<String, String>()

    /**
     * Record that [forFile] depends on [depPath]. Self-references are
     * skipped so a file isn't listed as its own dep.
     */
    fun recordDepPath(forFile: String, depPath: String) {
        if (depPath == forFile) return
        depPaths.getOrPut(forFile) { LinkedHashSet() }.add(depPath)
    }

    /**
     * Record a dependency class projection observed while analyzing
     * [forFile]. First-wins on duplicate FQNs within the same file —
     * supertype resolution can revisit the same class via multiple
     * paths but the projection is the same shape either way.
     */
    fun recordPerFileDep(forFile: String, fqn: String, payload: ClassPayload) {
        depsByFile.getOrPut(forFile) { LinkedHashMap() }.putIfAbsent(fqn, payload)
    }

    /** Record a deterministic analysis crash for [forFile] with the given message. */
    fun recordCrash(forFile: String, error: String) {
        crashes[forFile] = error
    }

    /** Read-only snapshot of `depPathsByFile`. */
    val depPathsByFile: Map<String, Set<String>>
        get() = depPaths

    /** Read-only snapshot of `perFileDeps`. */
    val perFileDeps: Map<String, Map<String, ClassPayload>>
        get() = depsByFile

    /** Read-only snapshot of `crashedFiles`. */
    val crashedFiles: Map<String, String>
        get() = crashes

    /** The wire view of what was recorded. */
    fun view(): OracleResponse.CacheDepsView = OracleResponse.CacheDepsView(
        depPathsByFile = depPathsByFile,
        perFileDeps = perFileDeps,
        crashedFiles = crashedFiles,
    )

    /** True iff no per-file deps or crashes have been recorded yet. */
    fun isEmpty(): Boolean = depPaths.isEmpty() && depsByFile.isEmpty() && crashes.isEmpty()
}
