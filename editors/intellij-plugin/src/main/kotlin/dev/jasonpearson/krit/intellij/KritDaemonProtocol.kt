package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonElement
import com.google.gson.JsonObject
import com.google.gson.JsonParseException
import com.google.gson.annotations.SerializedName
import java.util.Base64

// Wire types for krit's line-delimited JSON daemon protocol. See
// `internal/daemon/protocol.go` on the Go side — verb names and
// envelope shape must stay in lockstep with that file.
object KritDaemonProtocol {
    const val VERB_ANALYZE_BUFFER = "analyze-buffer"
    const val VERB_ANALYZE_BUFFERS = "analyze-buffers"
    const val VERB_STATUS = "status"

    data class Request(
        val verb: String,
        val args: JsonObject?,
    )

    data class Response(
        val ok: Boolean = false,
        val error: String = "",
        val data: JsonObject? = null,
    )

    // Subset of the daemon's StatusResult the plugin reads. binary_hash is
    // the SHA-256 of the krit binary the daemon was started from.
    data class StatusData(
        @SerializedName("binary_hash") val binaryHash: String? = null,
    )

    data class AnalyzeBufferArgs(
        val path: String,
        val content: String,
    )

    data class AnalyzeBuffersArgs(
        val buffers: List<AnalyzeBufferArgs>,
    )

    // One slot per requested buffer, in request order. A buffer-level
    // failure sets error and leaves findings empty (AnalyzeBufferEntry).
    data class AnalyzeBufferEntry(
        @SerializedName("findings") val findings: JsonObject? = null,
        @SerializedName("cache_hit") val cacheHit: Boolean = false,
        @SerializedName("error") val error: String? = null,
        // Rule ID to fix level for each finding with an autofix; see
        // AnalyzeBufferData.fixLevels.
        @SerializedName("fix_levels") val fixLevels: Map<String, String>? = null,
    )

    data class AnalyzeBuffersData(
        @SerializedName("results") val results: List<AnalyzeBufferEntry>? = null,
        // IDs of the rules the buffer scan evaluated. Daemons that predate
        // the field leave it out.
        @SerializedName("rules") val rules: List<String>? = null,
    )

    data class AnalyzeBufferData(
        @SerializedName("findings") val findings: JsonObject? = null,
        @SerializedName("cache_hit") val cacheHit: Boolean = false,
        // Rule ID to the rule's fix level ("cosmetic", "idiomatic",
        // "semantic") for each finding with an autofix: the fixLevel the CLI
        // JSON reports, which the columns don't carry. Daemons that predate
        // the field leave it out.
        @SerializedName("fix_levels") val fixLevels: Map<String, String>? = null,
    )
}

// Decodes the columnar FindingColumns JSON the daemon emits into the
// flat KritFinding list the rest of the plugin already speaks. Mirrors
// `scanner.FindingColumns.MarshalJSON` (internal/scanner/findings_json.go)
// and `severityFromID` / `confidenceToByte` on the Go side. Keep this
// in sync with those when fields move.
object KritColumnarFindingsDecoder {
    private val gson = Gson()

    private const val SEVERITY_INFO: Int = 0
    private const val SEVERITY_WARNING: Int = 1
    private const val SEVERITY_ERROR: Int = 2

    // fixLevels maps rule IDs to fix levels (the daemon's fix_levels), so
    // fixable findings carry the same fixLevel as the CLI JSON. Throws
    // JsonParseException (a JsonSyntaxException from Gson itself) on a
    // malformed payload; the daemon callers in KritRunner then fall back to
    // the CLI.
    fun decode(json: JsonObject?, fixLevels: Map<String, String>? = null): List<KritFinding> {
        if (json == null) return emptyList()
        val columns = gson.fromJson(json, ColumnarPayload::class.java) ?: return emptyList()
        return columns.toFindings(fixLevels.orEmpty())
    }

    // Fix and SuggestedFix carry no json tags on the Go side, so their pool
    // entries use the Go field names.
    private data class PoolFix(
        @SerializedName("Safety") val safety: Int? = null,
    )

    private data class PoolSuggestedFix(
        @SerializedName("ID") val id: String? = null,
        @SerializedName("Title") val title: String? = null,
        @SerializedName("Detail") val detail: String? = null,
        @SerializedName("Edits") val edits: List<PoolSuggestedEdit>? = null,
        @SerializedName("ApplicationToken") val applicationToken: String? = null,
    ) {
        fun toSuggestedFix() = KritSuggestedFix(
            id = id.orEmpty(),
            title = title.orEmpty(),
            detail = detail.orEmpty(),
            edits = edits.orEmpty().map { it.toSuggestedEdit() },
            applicationToken = applicationToken.orEmpty(),
        )
    }

    private data class PoolSuggestedEdit(
        @SerializedName("TargetFile") val targetFile: String? = null,
        @SerializedName("StartLine") val startLine: Int = 0,
        @SerializedName("EndLine") val endLine: Int = 0,
        @SerializedName("StartByte") val startByte: Int = 0,
        @SerializedName("EndByte") val endByte: Int = 0,
        @SerializedName("ByteMode") val byteMode: Boolean = false,
        @SerializedName("Replacement") val replacement: String? = null,
    ) {
        fun toSuggestedEdit() = KritSuggestedEdit(
            targetFile = targetFile.orEmpty(),
            startLine = startLine,
            endLine = endLine,
            startByte = startByte,
            endByte = endByte,
            byteMode = byteMode,
            replacement = replacement.orEmpty(),
        )
    }

    private data class ColumnarPayload(
        val files: List<String> = emptyList(),
        val ruleSets: List<String> = emptyList(),
        val rules: List<String> = emptyList(),
        val messages: List<String> = emptyList(),
        val fileIdx: List<Int> = emptyList(),
        val line: List<Int> = emptyList(),
        val col: List<Int> = emptyList(),
        val ruleSetIdx: List<Int> = emptyList(),
        val ruleIdx: List<Int> = emptyList(),
        // Go marshals these []uint8 columns as base64 strings; a JSON array
        // of numbers is accepted too. See byteColumn.
        val severityID: JsonElement? = null,
        val messageIdx: List<Int> = emptyList(),
        val confidence: JsonElement? = null,
        // 1-based references into fixPool / suggestedFixPool; 0 means none.
        // suggestedFixStart and suggestedFixCount are left out when all zero.
        val fixPool: List<PoolFix>? = null,
        val fixStart: List<Int>? = null,
        val suggestedFixPool: List<PoolSuggestedFix>? = null,
        val suggestedFixStart: List<Int>? = null,
        val suggestedFixCount: List<Int>? = null,
        val n: Int = 0,
    ) {
        fun toFindings(fixLevels: Map<String, String>): List<KritFinding> {
            val rowCount = rowCount()
            if (rowCount == 0) return emptyList()
            val severities = byteColumn(severityID)
            val confidences = byteColumn(confidence)
            if (severities.isNotEmpty() && severities.size != rowCount) {
                throw JsonParseException("severityID has ${severities.size} rows, expected $rowCount")
            }
            if (confidences.isNotEmpty() && confidences.size != rowCount) {
                throw JsonParseException("confidence has ${confidences.size} rows, expected $rowCount")
            }
            return (0 until rowCount).map { row ->
                val rule = lookup(rules, ruleIdx, row).orEmpty()
                // Mirrors output.buildJSONFindings: fixable means a text
                // autofix; fixLevel is the rule's level, reported only then.
                val fix = fixAt(row)
                KritFinding(
                    file = lookup(files, fileIdx, row).orEmpty(),
                    line = line.getOrElse(row) { 0 },
                    column = col.getOrElse(row) { 0 },
                    ruleSet = lookup(ruleSets, ruleSetIdx, row).orEmpty(),
                    rule = rule,
                    severity = decodeSeverity(severities.getOrElse(row) { SEVERITY_INFO }),
                    message = lookup(messages, messageIdx, row).orEmpty(),
                    confidence = (confidences.getOrElse(row) { 0 }) / 100.0,
                    fixable = fix != null,
                    fixLevel = if (fix == null) null else fixLevels[rule] ?: safetyName(fix.safety),
                    suggestedFixes = suggestedFixesAt(row),
                )
            }
        }

        private fun fixAt(row: Int): PoolFix? {
            val ref = fixStart?.getOrNull(row) ?: return null
            if (ref <= 0) return null
            return fixPool?.getOrNull(ref - 1) ?: throw JsonParseException("fixStart out of range at row $row")
        }

        private fun suggestedFixesAt(row: Int): List<KritSuggestedFix> {
            val start = suggestedFixStart?.getOrNull(row) ?: return emptyList()
            val count = suggestedFixCount?.getOrNull(row) ?: return emptyList()
            if (start <= 0 || count <= 0) return emptyList()
            val pool = suggestedFixPool.orEmpty()
            if (start - 1 + count > pool.size) throw JsonParseException("suggested fix range out of bounds at row $row")
            return (start - 1 until start - 1 + count).map { pool[it].toSuggestedFix() }
        }

        // Fix.Safety (set by custom rules): 1 cosmetic, 2 idiomatic, 3 semantic.
        private fun safetyName(safety: Int?): String? = when (safety) {
            1 -> "cosmetic"
            2 -> "idiomatic"
            3 -> "semantic"
            else -> null
        }

        private fun rowCount(): Int = if (n > 0) n else fileIdx.size

        private fun lookup(pool: List<String>, indexes: List<Int>, row: Int): String? {
            val idx = indexes.getOrNull(row) ?: return null
            return pool.getOrNull(idx)
        }

        // encoding/json writes a []byte (and so []uint8) as a base64 string,
        // not an array; values are unsigned bytes. A malformed column throws
        // JsonParseException rather than decoding to defaults, so the caller
        // falls back to the CLI instead of publishing wrong findings.
        private fun byteColumn(element: JsonElement?): List<Int> = when {
            element == null || element.isJsonNull -> emptyList()
            element.isJsonArray -> element.asJsonArray.map { it.asInt }
            element.isJsonPrimitive && element.asJsonPrimitive.isString ->
                try {
                    Base64.getDecoder().decode(element.asString).map { it.toInt() and 0xFF }
                } catch (e: IllegalArgumentException) {
                    throw JsonParseException("malformed base64 byte column", e)
                }
            else -> throw JsonParseException("unexpected byte column: $element")
        }

        private fun decodeSeverity(id: Int): String = when (id) {
            SEVERITY_ERROR -> "error"
            SEVERITY_WARNING -> "warning"
            else -> "info"
        }
    }
}
