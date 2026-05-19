package dev.jasonpearson.krit.fir.plugins

/**
 * JSON wire-shape builders for the plugin-rule daemon RPCs. The shape
 * mirrors krit-types' `buildListPluginsResponse` so a single Go-side
 * client parses either backend's response with one struct.
 */
internal object PluginResponse {

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
