package dev.jasonpearson.krit.intellij

object KritFixLabels {
    private const val DEFAULT_FIX_LEVEL = "idiomatic"

    fun normalizeFixLevel(fixLevel: String?): String =
        fixLevel.orEmpty().ifBlank { DEFAULT_FIX_LEVEL }

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
