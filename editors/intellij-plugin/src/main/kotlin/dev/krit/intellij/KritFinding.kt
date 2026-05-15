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
)

data class KritReport(
    val findings: List<KritFinding> = emptyList(),
)

