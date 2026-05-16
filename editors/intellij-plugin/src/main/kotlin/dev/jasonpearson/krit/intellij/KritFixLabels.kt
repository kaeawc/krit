package dev.jasonpearson.krit.intellij

object KritFixLabels {
    private const val DEFAULT_FIX_LEVEL = "idiomatic"

    fun normalizeFixLevel(fixLevel: String?): String =
        fixLevel.orEmpty().ifBlank { DEFAULT_FIX_LEVEL }

    fun applyFixesTitle(fixLevel: String?): String =
        "Apply Krit ${normalizeFixLevel(fixLevel)} auto-fixes"

    fun suggestionTitle(suggestion: KritSuggestedFix): String {
        val title = suggestion.title.ifBlank { suggestion.id }
        return "Krit suggestion: $title"
    }

    const val SUGGESTION_FAMILY_NAME = "Krit suggested fix"
}
