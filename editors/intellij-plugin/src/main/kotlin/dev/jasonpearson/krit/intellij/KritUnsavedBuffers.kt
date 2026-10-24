package dev.jasonpearson.krit.intellij

// A project scan runs the krit CLI, which reads files from disk, so it misses
// edits the user hasn't saved. After the scan, the plugin sends the unsaved
// buffers to the daemon and lays those findings over the disk results.
object KritUnsavedBuffers {
    // Returns base with each buffer's findings replacing that file's disk
    // findings, or null when the buffer scan was cancelled. outcomes and base
    // are keyed by canonical path, and canonical maps a finding's file (which
    // may be relative to the project) to that form. A buffer the daemon could
    // not analyze (no outcome, or a Failure) keeps its disk findings: the user
    // sees the last saved state rather than nothing.
    fun overlay(
        base: Map<String, List<KritFinding>>,
        outcomes: Map<String, KritRunner.AnalyzeOutcome>,
        canonical: (String) -> String,
    ): Map<String, List<KritFinding>>? {
        if (outcomes.isEmpty()) return base
        val merged = base.toMutableMap()
        for ((path, outcome) in outcomes) {
            when (outcome) {
                is KritRunner.AnalyzeOutcome.Cancelled -> return null
                is KritRunner.AnalyzeOutcome.Success -> {
                    val own = outcome.report.findings.filter { canonical(it.file) == path }
                    if (own.isEmpty()) merged.remove(path) else merged[path] = own
                }
                else -> Unit
            }
        }
        return merged
    }
}
