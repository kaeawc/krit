package dev.jasonpearson.krit.fir.oracle

/**
 * Daemon-response builders for the oracle-style RPC methods (`analyze`,
 * `analyzeAll`, `analyzeFiles`, `analyzeWithDeps`). The wire shape
 * mirrors `buildDaemonResponse` in krit-types' `Main.kt` so a single
 * Go-side client can talk to either backend.
 *
 * The current builder emits the envelope shell with empty `files` and
 * `dependencies` maps — well-formed JSON that parses cleanly on the Go
 * side, but conveys no per-file facts. Per-file projection (class
 * declarations, members, expressions, annotations, diagnostics) is not
 * yet implemented; consumers see an empty result until each projection
 * lands.
 */
object OracleResponse {

    /**
     * Build the krit-types-compatible envelope for an analyze response.
     * `errors`, when non-empty, is nested inside `result` to match the
     * legacy builder shape (krit-types' `buildDaemonResponse`); the
     * flat `cacheDeps`-sibling shape used by `analyzeWithDeps` is
     * separate and lands with the `cacheDeps` projection in a later PR.
     */
    fun buildEmptyAnalyze(id: Long, errors: Map<String, String> = emptyMap()): String {
        val sb = StringBuilder()
        sb.append("""{"id":""")
        sb.append(id)
        sb.append(""","result":{""")
        sb.append(""""version":1,""")
        sb.append(""""kotlinVersion":""")
        sb.append(jsonString(KotlinVersion.CURRENT.toString()))
        sb.append(""","files":{},""")
        sb.append(""""dependencies":{}""")
        if (errors.isNotEmpty()) {
            sb.append(""","errors":{""")
            errors.entries.forEachIndexed { i, (path, msg) ->
                if (i > 0) sb.append(",")
                sb.append(jsonString(path))
                sb.append(":")
                sb.append(jsonString(msg))
            }
            sb.append("}")
        }
        sb.append("}}")
        return sb.toString()
    }

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
