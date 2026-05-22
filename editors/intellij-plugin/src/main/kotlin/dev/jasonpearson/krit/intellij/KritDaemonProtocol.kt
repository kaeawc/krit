package dev.jasonpearson.krit.intellij

import com.google.gson.Gson
import com.google.gson.JsonObject
import com.google.gson.annotations.SerializedName

// Wire types for krit's line-delimited JSON daemon protocol. See
// `internal/daemon/protocol.go` on the Go side — verb names and
// envelope shape must stay in lockstep with that file.
object KritDaemonProtocol {
    const val VERB_ANALYZE_BUFFER = "analyze-buffer"
    const val VERB_ANALYZE_BUFFERS = "analyze-buffers"

    data class Request(
        val verb: String,
        val args: JsonObject?,
    )

    data class Response(
        val ok: Boolean = false,
        val error: String = "",
        val data: JsonObject? = null,
    )

    data class AnalyzeBufferArgs(
        val path: String,
        val content: String,
    )

    data class AnalyzeBufferData(
        @SerializedName("findings") val findings: JsonObject? = null,
        @SerializedName("cache_hit") val cacheHit: Boolean = false,
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

    fun decode(json: JsonObject?): List<KritFinding> {
        if (json == null) return emptyList()
        val columns = gson.fromJson(json, ColumnarPayload::class.java) ?: return emptyList()
        return columns.toFindings()
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
        val severityID: List<Int> = emptyList(),
        val messageIdx: List<Int> = emptyList(),
        val confidence: List<Int> = emptyList(),
        val n: Int = 0,
    ) {
        fun toFindings(): List<KritFinding> {
            val rowCount = rowCount()
            if (rowCount == 0) return emptyList()
            return (0 until rowCount).map { row ->
                KritFinding(
                    file = lookup(files, fileIdx, row).orEmpty(),
                    line = line.getOrElse(row) { 0 },
                    column = col.getOrElse(row) { 0 },
                    ruleSet = lookup(ruleSets, ruleSetIdx, row).orEmpty(),
                    rule = lookup(rules, ruleIdx, row).orEmpty(),
                    severity = decodeSeverity(severityID.getOrElse(row) { SEVERITY_INFO }),
                    message = lookup(messages, messageIdx, row).orEmpty(),
                    confidence = (confidence.getOrElse(row) { 0 }) / 100.0,
                )
            }
        }

        private fun rowCount(): Int = if (n > 0) n else fileIdx.size

        private fun lookup(pool: List<String>, indexes: List<Int>, row: Int): String? {
            val idx = indexes.getOrNull(row) ?: return null
            return pool.getOrNull(idx)
        }

        private fun decodeSeverity(id: Int): String = when (id) {
            SEVERITY_ERROR -> "error"
            SEVERITY_WARNING -> "warning"
            else -> "info"
        }
    }
}
