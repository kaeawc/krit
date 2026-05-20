package dev.jasonpearson.krit.fir.plugins

/**
 * JSON wire-shape builders for the plugin-rule daemon RPCs. The shape
 * mirrors krit-types' `buildListPluginsResponse` so a single Go-side
 * client parses either backend's response with one struct.
 */
internal object PluginResponse {

    /**
     * Build the `analyzeFileWithPlugins` response. Mirrors krit-types'
     * `buildAnalyzeFileResponse`: a `findings` array (with optional
     * per-finding `fix` metadata) plus an optional `errors` map keyed
     * by rule ID for rules that threw mid-`check`.
     */
    fun buildAnalyzeFile(
        id: Long,
        findings: List<PluginRuleRunner.PluginFinding>,
        errors: Map<String, String>,
    ): String {
        val sb = StringBuilder()
        sb.append("""{"id":""").append(id).append(""","result":{"findings":[""")
        findings.forEachIndexed { i, f ->
            if (i > 0) sb.append(',')
            sb.append('{')
            sb.append("\"file\":").append(jsonString(f.file)).append(',')
            sb.append("\"line\":").append(f.line).append(',')
            sb.append("\"column\":").append(f.column).append(',')
            sb.append("\"startByte\":").append(f.startByte).append(',')
            sb.append("\"endByte\":").append(f.endByte).append(',')
            sb.append("\"ruleSet\":").append(jsonString(f.ruleSet)).append(',')
            sb.append("\"ruleId\":").append(jsonString(f.ruleId)).append(',')
            sb.append("\"severity\":").append(jsonString(f.severity)).append(',')
            sb.append("\"message\":").append(jsonString(f.message)).append(',')
            sb.append("\"confidence\":").append(f.confidence)
            f.fix?.let { fix ->
                sb.append(",\"fix\":{")
                sb.append("\"startLine\":").append(fix.startLine).append(',')
                sb.append("\"endLine\":").append(fix.endLine).append(',')
                sb.append("\"replacement\":").append(jsonString(fix.replacement)).append(',')
                sb.append("\"safety\":").append(jsonString(fix.safety))
                sb.append('}')
            }
            sb.append('}')
        }
        sb.append(']')
        if (errors.isNotEmpty()) {
            sb.append(",\"errors\":{")
            errors.entries.forEachIndexed { i, (ruleId, msg) ->
                if (i > 0) sb.append(',')
                sb.append(jsonString(ruleId)).append(':').append(jsonString(msg))
            }
            sb.append('}')
        }
        sb.append("}}")
        return sb.toString()
    }

    fun buildListPlugins(
        id: Long,
        descriptors: List<PluginRuleDescriptor>,
        diagnostics: List<PluginLoadDiagnostic>,
    ): String {
        val sb = StringBuilder()
        sb.append("""{"id":""").append(id).append(""","result":{"rules":[""")
        descriptors.forEachIndexed { i, descriptor ->
            if (i > 0) sb.append(',')
            sb.append('{')
            sb.append("\"ruleId\":").append(jsonString(descriptor.ruleId)).append(',')
            sb.append("\"category\":").append(jsonString(descriptor.category)).append(',')
            sb.append("\"severity\":").append(jsonString(descriptor.severity)).append(',')
            sb.append("\"maturity\":").append(jsonString(descriptor.maturity)).append(',')
            appendStringArray(sb, "languages", descriptor.languages)
            sb.append(',')
            appendStringArray(sb, "needs", descriptor.needs)
            if (descriptor.sdkVersion.isNotBlank()) {
                sb.append(",\"sdkVersion\":").append(jsonString(descriptor.sdkVersion))
            }
            sb.append('}')
        }
        sb.append(']')
        appendDiagnostics(sb, diagnostics)
        sb.append('}').append('}')
        return sb.toString()
    }

    private fun appendDiagnostics(sb: StringBuilder, diagnostics: List<PluginLoadDiagnostic>) {
        if (diagnostics.isEmpty()) return
        sb.append(",\"diagnostics\":[")
        diagnostics.forEachIndexed { i, d ->
            if (i > 0) sb.append(',')
            sb.append('{')
            sb.append("\"jar\":").append(jsonString(d.jar)).append(',')
            sb.append("\"level\":").append(jsonString(d.level.name.lowercase())).append(',')
            sb.append("\"ruleSdkVersion\":").append(jsonString(d.ruleSdkVersion)).append(',')
            sb.append("\"daemonSdkVersion\":").append(jsonString(d.daemonSdkVersion)).append(',')
            sb.append("\"message\":").append(jsonString(d.message))
            sb.append('}')
        }
        sb.append(']')
    }

    private fun appendStringArray(sb: StringBuilder, key: String, values: List<String>) {
        sb.append('"').append(key).append("\":[")
        values.forEachIndexed { i, v ->
            if (i > 0) sb.append(',')
            sb.append(jsonString(v))
        }
        sb.append(']')
    }

    /**
     * Minimal JSON string escaper for plugin-response payloads. Mirrors
     * the escape set used by `OracleResponse.jsonString`; we don't share
     * the function because the two builders evolve independently and
     * exposing a shared helper means publishing one of them as public.
     */
    private fun jsonString(value: String): String {
        val sb = StringBuilder(value.length + 2)
        sb.append('"')
        for (c in value) {
            when {
                c == '"' -> sb.append("\\\"")
                c == '\\' -> sb.append("\\\\")
                c == '\n' -> sb.append("\\n")
                c == '\r' -> sb.append("\\r")
                c == '\t' -> sb.append("\\t")
                c.code < 0x20 -> sb.append("\\u%04x".format(c.code))
                else -> sb.append(c)
            }
        }
        sb.append('"')
        return sb.toString()
    }
}
