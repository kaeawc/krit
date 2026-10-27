package dev.jasonpearson.krit.intellij

/**
 * How a finding is shown, in plugin terms. Maps to IntelliJ's
 * HighlightSeverity / ProblemHighlightType / HighlightDisplayLevel in
 * [KritSeverity].
 */
enum class KritLevel { ERROR, WARNING, WEAK_WARNING, INFORMATION }

/**
 * Per-rule filtering and severity for findings, from the per-rule
 * inspections (`Krit_<RuleId>`) in the active inspection profile. Free of
 * IntelliJ classes: the profile is read through [RuleSetting] lookups.
 *
 * Precedence (also in the plugin README):
 * - krit.yml (or the configured config file) decides which rules run and the
 *   severity krit reports. The IDE cannot turn on a rule krit.yml disables.
 * - A per-rule inspection turned off in the profile hides that rule's
 *   findings; krit still computes them.
 * - A per-rule inspection left at its default level shows the severity krit
 *   reported (so krit.yml severity overrides still apply). Once the level is
 *   changed in the profile, the profile's level wins.
 * - A rule with no per-rule inspection (the rule list couldn't be read, or
 *   a rule newer than the list) is shown as krit reports it.
 */
object KritRuleFilter {
    const val SHORT_NAME_PREFIX = "Krit_"

    fun shortNameFor(ruleId: String): String = SHORT_NAME_PREFIX + ruleId

    /**
     * The profile's state for one rule's inspection: whether it is on (for
     * the file at hand, scopes included) and its level, with [customized]
     * true when the level differs from the inspection's default.
     */
    data class RuleSetting(val enabled: Boolean, val level: KritLevel?, val customized: Boolean)

    /** A finding with the level to show it at. */
    data class Shown(val finding: KritFinding, val level: KritLevel)

    /**
     * The level krit's registry severity maps to, used as the default level
     * of the rule's inspection.
     */
    fun defaultLevel(severity: String): KritLevel = when {
        severity.equals("error", ignoreCase = true) -> KritLevel.ERROR
        severity.equals("info", ignoreCase = true) -> KritLevel.INFORMATION
        else -> KritLevel.WARNING
    }

    // Confidence in (0, 0.5) is a "soft" finding — render at weak warning so
    // it doesn't compete with high-confidence diagnostics. Confidence == 0
    // means the rule didn't set it; treat as a normal warning.
    private const val WEAK_THRESHOLD = 0.5

    /** The level krit's reported severity (and confidence) maps to. */
    fun reportedLevel(finding: KritFinding): KritLevel = when {
        finding.severity.equals("error", ignoreCase = true) -> KritLevel.ERROR
        finding.severity.equals("info", ignoreCase = true) -> KritLevel.INFORMATION
        finding.confidence > 0.0 && finding.confidence < WEAK_THRESHOLD -> KritLevel.WEAK_WARNING
        else -> KritLevel.WARNING
    }

    /**
     * The level to show [finding] at given its rule's [setting], or null
     * when the rule's inspection is off. A null [setting] (no per-rule
     * inspection) shows the reported level.
     */
    fun levelFor(finding: KritFinding, setting: RuleSetting?): KritLevel? {
        if (setting == null) return reportedLevel(finding)
        if (!setting.enabled) return null
        if (setting.customized && setting.level != null) return setting.level
        return reportedLevel(finding)
    }

    /** [findings] that should be shown, each with its level. */
    fun apply(findings: List<KritFinding>, settingFor: (ruleId: String) -> RuleSetting?): List<Shown> {
        if (findings.isEmpty()) return emptyList()
        val cache = HashMap<String, RuleSetting?>()
        return findings.mapNotNull { finding ->
            val setting = cache.getOrPut(finding.rule) { settingFor(finding.rule) }
            levelFor(finding, setting)?.let { Shown(finding, it) }
        }
    }
}
