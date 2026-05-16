package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonSyntaxException

object KritJsonParser {
    private val gson = Gson()

    fun parse(json: String): KritReport {
        if (json.isBlank()) {
            return KritReport()
        }
        // Gson uses reflective field assignment, so Kotlin default values do
        // not apply to missing JSON fields — reference-typed fields land as
        // null even though their declared type is non-null. Normalize after
        // parse so the rest of the plugin sees a well-typed model.
        val raw = try {
            gson.fromJson(json, KritReport::class.java) ?: KritReport()
        } catch (_: JsonSyntaxException) {
            KritReport()
        }
        return normalize(raw)
    }

    private fun normalize(report: KritReport): KritReport {
        val findings = report.findings.orNull().map { finding ->
            finding.copy(
                suggestedFixes = finding.suggestedFixes.orNull().map { normalizeSuggestion(it) },
            )
        }
        return report.copy(findings = findings)
    }

    private fun normalizeSuggestion(suggestion: KritSuggestedFix): KritSuggestedFix {
        return suggestion.copy(
            id = suggestion.id.orNull(),
            title = suggestion.title.orNull(),
            detail = suggestion.detail.orNull(),
            applicationToken = suggestion.applicationToken.orNull(),
            edits = suggestion.edits.orNull().map { normalizeEdit(it) },
        )
    }

    private fun normalizeEdit(edit: KritSuggestedEdit): KritSuggestedEdit {
        return edit.copy(
            targetFile = edit.targetFile.orNull(),
            replacement = edit.replacement.orNull(),
        )
    }

    @Suppress("UNCHECKED_CAST")
    private fun <T> List<T>.orNull(): List<T> = (this as List<T>?) ?: emptyList()

    private fun String.orNull(): String = (this as String?) ?: ""
}
