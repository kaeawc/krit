package dev.jasonpearson.krit.intellij

object KritFixLabels {
    private const val DEFAULT_FIX_LEVEL = "idiomatic"

    // krit's fix safety tiers, least to most invasive (rules.FixLevel).
    val FIX_LEVELS = listOf("cosmetic", "idiomatic", "semantic")

    fun normalizeFixLevel(fixLevel: String?): String =
        fixLevel.orEmpty().ifBlank { DEFAULT_FIX_LEVEL }

    // Index of a known level in FIX_LEVELS, or -1.
    private fun rank(fixLevel: String?): Int = FIX_LEVELS.indexOf(fixLevel.orEmpty().trim().lowercase())

    // A known maximum, else the default (a hand-edited krit.xml could hold
    // anything).
    private fun normalizeMax(maxFixLevel: String?): String =
        FIX_LEVELS.getOrNull(rank(maxFixLevel)) ?: DEFAULT_FIX_LEVEL

    /**
     * Whether a fix at [fixLevel] may be applied under [maxFixLevel] (the
     * "Maximum fix level" setting). A finding that doesn't state a known
     * level is allowed: it is applied at the maximum, which is what krit's
     * --fix-level means.
     */
    fun isWithin(fixLevel: String?, maxFixLevel: String?): Boolean {
        val level = rank(fixLevel)
        return level < 0 || level <= rank(normalizeMax(maxFixLevel))
    }

    /** The --fix-level to run a finding's fix with: its own level, else [maxFixLevel]. */
    fun levelToApply(fixLevel: String?, maxFixLevel: String?): String =
        FIX_LEVELS.getOrNull(rank(fixLevel)) ?: normalizeMax(maxFixLevel)

    // Per-finding label — kept short because IntelliJ truncates intention
    // text in the editor gutter. Avoids any "all" / "project" wording so
    // the user doesn't expect a project-wide rewrite from a single click.
    fun applyFixTitle(fixLevel: String?): String =
        "Apply Krit ${normalizeFixLevel(fixLevel)} fix"

    fun suggestionTitle(suggestion: KritSuggestedFix): String {
        val title = suggestion.title.ifBlank { suggestion.id }
        return "Krit suggestion: $title"
    }

    const val SUGGESTION_FAMILY_NAME = "Krit suggested fix"
}
