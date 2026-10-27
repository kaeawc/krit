package dev.jasonpearson.krit.intellij

import java.io.File

/**
 * Which files to rescan after a fix. Each function returns the absolute paths
 * the fix can have written, or null when that can't be told, in which case the
 * caller rescans the whole project. Kept free of IntelliJ classes so it is
 * unit-testable without the platform.
 */
object KritFixTargets {
    /**
     * `krit --fix --finding-id ID` strips every text fix but that finding's
     * (pipeline.FixupInput.OnlyFindingID) and applies no binary fixes, so it
     * writes only the finding's own file.
     */
    fun forFix(findings: Iterable<KritFinding>, findingId: String, basePath: String?): Set<String>? {
        val finding = findings.firstOrNull { it.findingId == findingId } ?: return null
        val path = resolve(finding.file, basePath) ?: return null
        return setOf(path)
    }

    /**
     * `krit apply-suggestion` writes each edit's targetFile (resolved against
     * --base when relative), which need not be the finding's file.
     */
    fun forSuggestion(
        findings: Iterable<KritFinding>,
        findingId: String,
        suggestionId: String,
        basePath: String?,
    ): Set<String>? {
        val finding = findings.firstOrNull { it.findingId == findingId } ?: return null
        val suggestion = finding.suggestedFixes.firstOrNull { it.id == suggestionId } ?: return null
        if (suggestion.edits.isEmpty()) return null
        val targets = LinkedHashSet<String>()
        for (edit in suggestion.edits) {
            targets += resolve(edit.targetFile, basePath) ?: return null
        }
        return targets
    }

    private fun resolve(path: String, basePath: String?): String? {
        if (path.isBlank()) return null
        val file = File(path)
        val absolute = when {
            file.isAbsolute -> file
            basePath.isNullOrBlank() -> return null
            else -> File(basePath, path)
        }
        return absolute.toPath().normalize().toString()
    }

    /** The rule of the finding with [findingId], or null when it isn't among [findings]. */
    fun ruleOf(findings: Iterable<KritFinding>, findingId: String): String? =
        findings.firstOrNull { it.findingId == findingId }?.rule

    /**
     * Whether a per-file rescan can stand in for a project scan after fixing
     * a finding of [fixedRule]: only when the scan re-evaluates that rule.
     * A buffer scan runs only per-file rules ([evaluatedRules]); a
     * project-only rule's fixed finding (DeadCode, say) would otherwise
     * survive the rescan. Unknown rule or rule set means a project scan.
     */
    fun perFileRescanCovers(fixedRule: String?, evaluatedRules: Set<String>?): Boolean =
        fixedRule != null && !evaluatedRules.isNullOrEmpty() && fixedRule in evaluatedRules
}
