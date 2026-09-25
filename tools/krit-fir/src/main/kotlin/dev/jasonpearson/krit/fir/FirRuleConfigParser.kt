package dev.jasonpearson.krit.fir

import dev.jasonpearson.krit.fir.plugins.PayloadParsers

/** Small escape-aware JSON reader for check.ruleConfigs, without a JSON dependency. */
internal fun parseFirRuleConfigs(json: String): Map<String, Map<String, Any?>> {
    val block = PayloadParsers.extractObjectBlock(json, "ruleConfigs") ?: return emptyMap()
    val parsed = ConfigJsonReader(block).value() as? Map<*, *> ?: return emptyMap()
    return parsed.mapNotNull { (id, options) ->
        val key = id as? String ?: return@mapNotNull null
        val values = options as? Map<*, *> ?: return@mapNotNull null
        key to values.entries.mapNotNull { (name, value) ->
            (name as? String)?.let { it to value }
        }.toMap()
    }.toMap()
}

private class ConfigJsonReader(private val text: String) {
    private var pos = 0
    fun value(): Any? {
        space()
        return when (text.getOrNull(pos)) {
            '{' -> objectValue()
            '[' -> arrayValue()
            '"' -> stringValue()
            else -> scalar()
        }
    }
    private fun objectValue(): Map<String, Any?> {
        pos++
        val out = linkedMapOf<String, Any?>()
        space()
        while (text.getOrNull(pos) != '}') {
            val key = stringValue()
            space()
            require(text.getOrNull(pos++) == ':') { "Malformed ruleConfigs object" }
            out[key] = value()
            space()
            if (text.getOrNull(pos) != ',') break
            pos++
            space()
        }
        require(text.getOrNull(pos++) == '}') { "Malformed ruleConfigs object" }
        return out
    }
    private fun arrayValue(): List<Any?> {
        pos++
        val out = mutableListOf<Any?>()
        space()
        while (text.getOrNull(pos) != ']') {
            out += value()
            space()
            if (text.getOrNull(pos) != ',') break
            pos++
        }
        require(text.getOrNull(pos++) == ']') { "Malformed ruleConfigs array" }
        return out
    }
    private fun stringValue(): String {
        require(text.getOrNull(pos++) == '"') { "Malformed ruleConfigs string" }
        val out = StringBuilder()
        while (pos < text.length) {
            val c = text[pos++]
            if (c == '"') return out.toString()
            if (c != '\\') { out.append(c); continue }
            val escape = text[pos++]
            out.append(when (escape) {
                '"', '\\', '/' -> escape
                'n' -> '\n'
                'r' -> '\r'
                't' -> '\t'
                'b' -> '\b'
                'f' -> '\u000c'
                'u' -> text.substring(pos, pos + 4).also { pos += 4 }.toInt(16).toChar()
                else -> throw IllegalArgumentException("Malformed ruleConfigs escape")
            })
        }
        throw IllegalArgumentException("Unterminated ruleConfigs string")
    }
    private fun scalar(): Any? {
        val start = pos
        while (pos < text.length && text[pos] !in ",]} \t\r\n") pos++
        val token = text.substring(start, pos)
        return when (token) {
            "true" -> true
            "false" -> false
            "null" -> null
            else -> token.toLongOrNull() ?: token.toDoubleOrNull()
                ?: throw IllegalArgumentException("Malformed ruleConfigs value: $token")
        }
    }
    private fun space() { while (text.getOrNull(pos)?.isWhitespace() == true) pos++ }
}
