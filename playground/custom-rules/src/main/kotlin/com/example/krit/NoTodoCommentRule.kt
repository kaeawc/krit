package com.example.krit

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity

/**
 * Flags `// TODO`, `// FIXME`, and bare `TODO(...)` calls left in production
 * Kotlin sources. The rule scans line-by-line and skips strings, raw strings,
 * and block comments to avoid the most common lexical false positives.
 */
@KritRuleInfo(
    id = "playground.NoTodoComment",
    category = "playground-style",
    severity = Severity.WARNING,
)
class NoTodoCommentRule : KritRule {

    private val lineCommentMarker = Regex("""//\s*(TODO|FIXME)\b""")
    private val bareTodoCall = Regex("""\bTODO\s*\(""")

    override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        val findings = mutableListOf<Finding>()
        var inBlockComment = false

        file.text.lineSequence().forEachIndexed { idx, raw ->
            val line = stripStrings(raw)

            if (inBlockComment) {
                val end = line.indexOf("*/")
                if (end < 0) return@forEachIndexed
                inBlockComment = false
                val tail = line.substring(end + 2)
                report(tail, idx, raw, findings)
                return@forEachIndexed
            }

            val blockStart = line.indexOf("/*")
            val toScan = if (blockStart >= 0) {
                val rest = line.substring(blockStart + 2)
                val end = rest.indexOf("*/")
                if (end < 0) {
                    inBlockComment = true
                }
                line.substring(0, blockStart) + if (end >= 0) rest.substring(end + 2) else ""
            } else {
                line
            }

            report(toScan, idx, raw, findings)
        }
        return findings
    }

    private fun report(scan: String, idx: Int, raw: String, into: MutableList<Finding>) {
        lineCommentMarker.find(scan)?.let { m ->
            into.add(
                Finding(
                    message = "Unresolved ${m.groupValues[1]} comment.",
                    line = idx + 1,
                    column = raw.indexOf("//") + 1,
                    confidence = 0.95,
                )
            )
            return
        }
        bareTodoCall.find(scan)?.let { m ->
            into.add(
                Finding(
                    message = "Unimplemented `TODO(...)` call.",
                    line = idx + 1,
                    column = m.range.first + 1,
                    confidence = 0.95,
                )
            )
        }
    }

    /**
     * Replaces string-literal contents with spaces so embedded `// TODO`
     * inside a string never trips the rule. Triple-quoted strings are
     * approximated by zeroing out their contents on the same line.
     */
    private fun stripStrings(line: String): String {
        val sb = StringBuilder(line.length)
        var i = 0
        var inString = false
        while (i < line.length) {
            val c = line[i]
            when {
                !inString && i + 2 < line.length && line.regionMatches(i, "\"\"\"", 0, 3) -> {
                    sb.append("   ")
                    i += 3
                    val end = line.indexOf("\"\"\"", i)
                    if (end < 0) {
                        repeat(line.length - i) { sb.append(' ') }
                        return sb.toString()
                    }
                    repeat(end - i) { sb.append(' ') }
                    sb.append("   ")
                    i = end + 3
                }
                !inString && c == '"' -> {
                    inString = true
                    sb.append('"')
                    i++
                }
                inString && c == '\\' && i + 1 < line.length -> {
                    sb.append("  ")
                    i += 2
                }
                inString && c == '"' -> {
                    inString = false
                    sb.append('"')
                    i++
                }
                inString -> {
                    sb.append(' ')
                    i++
                }
                else -> {
                    sb.append(c)
                    i++
                }
            }
        }
        return sb.toString()
    }
}
