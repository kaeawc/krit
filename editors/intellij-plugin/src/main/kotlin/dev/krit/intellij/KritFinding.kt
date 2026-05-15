package dev.krit.intellij

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
) {
    val displayMessage: String
        get() = "Krit ${ruleSet}/${rule}: ${message}"
}

data class KritReport(
    val findings: List<KritFinding> = emptyList(),
)
