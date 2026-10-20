package dev.jasonpearson.krit.fir.incremental

import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.runner.Finding

/**
 * One file's last verdict, as a check request reports it: its findings (the path is whatever spelling the compile
 * used; the request's own spelling is applied when a result is assembled), the first compiler error that gates the
 * file, and the rules whose checker threw on it.
 */
internal class CachedVerdict(
    val findings: List<Finding>,
    val error: String?,
    val ruleErrors: Map<String, String>,
)

/**
 * The per-file verdicts of an [IncrementalState]'s module. An entry is valid for a request when the file's content
 * still hashes to [Entry.contentHash] and [Entry.verdictKey], the key of everything else the verdict depends on
 * (see [VerdictKeys]), equals the request's key for that file. It is touched only under the session's snapshot
 * lock, and only together with the state: [generation] is the state's generation after the last update, so a
 * cache that fell out of step with the state (a commit it did not see) is detected and not trusted.
 */
internal class VerdictCache {
    /**
     * [contentHash] is the SHA-1 the state recorded for the file; [suppliedHash] is the hash the request that last
     * compiled it carried in `FileRef.contentHash` (Go's own scheme, only comparable with itself), or null.
     */
    class Entry(val contentHash: String, val suppliedHash: String?, val verdictKey: String, val verdict: CachedVerdict)

    private val entries = HashMap<String, Entry>()

    /** The [IncrementalState.generation] this cache was last brought up to. */
    var generation: Int = -1
        private set

    val size: Int get() = entries.size

    fun get(file: String): Entry? = entries[file]

    /** A valid entry for [file] given its current [contentHash] and [verdictKey], or null. */
    fun valid(file: String, contentHash: String?, verdictKey: String): Entry? =
        entries[file]?.takeIf { contentHash != null && it.contentHash == contentHash && it.verdictKey == verdictKey }

    fun files(): Set<String> = HashSet(entries.keys)

    fun put(file: String, entry: Entry) { entries[file] = entry }

    fun remove(file: String) { entries.remove(file) }

    fun clear() { entries.clear(); generation = -1 }

    fun sync(stateGeneration: Int) { generation = stateGeneration }

    /** The first compile error a cached verdict records, files of [order] first. */
    fun firstError(order: Collection<String>): String? {
        for (file in order) entries[file]?.verdict?.error?.let { return it }
        return entries.values.firstNotNullOfOrNull { it.verdict.error }
    }
}

/**
 * Canonical-path views of a request's per-file inputs, and the keys derived from them. A verdict depends on the
 * rules and their configs (the same for every file of a request) and on the file's own entries in the test-file
 * set, the scan-path map and the SDK-level map. Paths are canonicalised so the key does not depend on how the
 * request spelled a file.
 */
internal class VerdictKeys(
    enabledRules: Set<String>,
    ruleConfigs: Map<String, Map<String, Any?>>,
    testFiles: Set<String>,
    scanPaths: Map<String, String>,
    sdkLevels: Map<String, SdkLevels>,
) {
    private val requestKey = FingerprintBuilder().add("rules", enabledRules).add("ruleConfigs", ruleConfigs).build()
    private val tests = testFiles.mapTo(HashSet()) { canonicalPath(it) }
    private val scans = scanPaths.entries.associate { canonicalPath(it.key) to it.value }
    private val sdks = sdkLevels.entries.associate { canonicalPath(it.key) to it.value }

    /** The key of [file]'s verdict (a canonical path). */
    fun of(file: String): String = FingerprintBuilder()
        .add("request", requestKey)
        .add("test", file in tests)
        .add("scan", scans[file])
        .add("sdk", sdks[file])
        .build()
}
