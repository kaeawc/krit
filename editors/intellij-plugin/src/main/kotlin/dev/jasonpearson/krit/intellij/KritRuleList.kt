package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonParseException

/**
 * One rule from `krit --list-rules --format json` (ListedRule in
 * internal/cli/scan/early_exits.go).
 */
data class KritRuleInfo(
    val id: String = "",
    val category: String = "",
    val severity: String = "",
    val defaultActive: Boolean = false,
    val fixable: Boolean = false,
    val fixLevel: String? = null,
    val description: String? = null,
    val plugin: Boolean = false,
)

/** Parses the `krit --list-rules --format json` document. Free of IntelliJ classes. */
object KritRuleList {
    private val gson = Gson()

    private class Document(val rules: List<KritRuleInfo>? = null)

    /**
     * The rules in [json], or null when it isn't the JSON rule list (an older
     * krit prints the text table for `--list-rules --format json`). Entries
     * without an id are dropped, as are repeated ids after the first, and
     * a blank category becomes "other".
     */
    fun parse(json: String): List<KritRuleInfo>? {
        if (json.isBlank()) return null
        val doc = try {
            gson.fromJson(json, Document::class.java)
        } catch (_: JsonParseException) {
            return null
        } catch (_: IllegalStateException) {
            return null
        } ?: return null
        // Gson skips Kotlin defaults: missing reference fields arrive as null.
        @Suppress("SENSELESS_COMPARISON")
        val rules = (doc.rules ?: return null).filter { it != null && it.id != null && it.id.isNotBlank() }
        val seen = HashSet<String>()
        return rules.filter { seen.add(it.id) }.map { rule ->
            @Suppress("SENSELESS_COMPARISON", "USELESS_ELVIS")
            rule.copy(
                category = (rule.category ?: "").ifBlank { "other" },
                severity = rule.severity ?: "",
            )
        }
    }

    /** Serializes [rules] in the same shape [parse] reads. */
    fun toJson(rules: List<KritRuleInfo>): String = gson.toJson(Document(rules))
}
