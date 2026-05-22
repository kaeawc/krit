package dev.jasonpearson.krit.intellij

data class KritFinding(
    val file: String,
    val line: Int,
    val column: Int,
    val ruleSet: String,
    val rule: String,
    val severity: String,
    val message: String,
    val fixable: Boolean = false,
    val fixLevel: String? = null,
    val confidence: Double = 0.0,
    val suggestedFixes: List<KritSuggestedFix> = emptyList(),
) {
    val displayMessage: String
        get() = "Krit ${ruleSet}/${rule}: ${message}"

    val findingId: String
        get() = "${rule}:${file}:${line}:${column}"
}

data class KritSuggestedFix(
    val id: String = "",
    val title: String = "",
    val detail: String = "",
    val edits: List<KritSuggestedEdit> = emptyList(),
    val applicationToken: String = "",
)

data class KritSuggestedEdit(
    val targetFile: String = "",
    val startLine: Int = 0,
    val endLine: Int = 0,
    val startByte: Int = 0,
    val endByte: Int = 0,
    val byteMode: Boolean = false,
    val replacement: String = "",
)

data class KritReport(
    val findings: List<KritFinding> = emptyList(),
)
