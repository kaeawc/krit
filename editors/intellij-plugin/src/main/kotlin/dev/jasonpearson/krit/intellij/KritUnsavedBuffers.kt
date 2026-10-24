package dev.jasonpearson.krit.intellij

// A project scan runs the krit CLI, which reads files from disk, so it misses
// edits the user hasn't saved. After the scan, the plugin sends the unsaved
// buffers to the daemon and lays those findings over the disk results.
object KritUnsavedBuffers {
    // Returns base with each buffer's findings laid over that file's disk
    // findings, or null when the buffer scan was cancelled. outcomes and base
    // are keyed by canonical path, and canonical maps a finding's file (which
    // may be relative to the project) to that form.
    //
    // The daemon runs only per-file rules on a buffer; cross-file, module,
    // Android, and Gradle rules need the whole project. So a buffer result
    // replaces only the disk findings of the rules in evaluatedRules, and the
    // project scan's findings for every other rule stay. A daemon that
    // doesn't report evaluatedRules (null or empty) leaves base unchanged,
    // as does a buffer the daemon could not analyze (no outcome, or a
    // Failure): the user sees the last saved state rather than a gap.
    fun overlay(
        base: Map<String, List<KritFinding>>,
        outcomes: Map<String, KritRunner.AnalyzeOutcome>,
        evaluatedRules: Set<String>?,
        canonical: (String) -> String,
    ): Map<String, List<KritFinding>>? {
        if (outcomes.values.any { it is KritRunner.AnalyzeOutcome.Cancelled }) return null
        if (outcomes.isEmpty() || evaluatedRules.isNullOrEmpty()) return base
        val merged = base.toMutableMap()
        for ((path, outcome) in outcomes) {
            if (outcome !is KritRunner.AnalyzeOutcome.Success) continue
            val kept = base[path].orEmpty().filter { it.rule !in evaluatedRules }
            val fromBuffer = outcome.report.findings.filter { canonical(it.file) == path }
            val combined = kept + fromBuffer
            if (combined.isEmpty()) merged.remove(path) else merged[path] = combined
        }
        return merged
    }
}
