package dev.jasonpearson.krit.fir.support

// Ports of Go's shared SQL-shape text helpers (sqlStaticOperand,
// splitSQLConcatOperands, sqlInterpolationUsesOnlyStaticSchemaConstants,
// sqlLastIdentifierSegment, sqlSchemaConstantName), for checkers that must
// classify a query or command string exactly as the Go rules do.

/**
 * Go's splitSQLConcatOperands: [text] split on `+` outside string literals
 * and parentheses; an empty list when there is no top-level `+`.
 */
fun splitSqlConcatOperands(text: String): List<String> {
    val out = mutableListOf<String>()
    var start = 0
    scanSqlOutsideStrings(text) { i, ch, depth ->
        if (ch == '+' && depth == 0) {
            out += text.substring(start, i).trim()
            start = i + 1
        }
        false
    }
    if (out.isEmpty()) return emptyList()
    out += text.substring(start).trim()
    return out
}

/**
 * Walks [text] the way Go's splitSQLConcatOperands does, calling [visit] with
 * each character outside string literals (plain or raw) and the parenthesis
 * depth after it; stops when [visit] returns true.
 */
inline fun scanSqlOutsideStrings(text: String, visit: (Int, Char, Int) -> Boolean) {
    var depth = 0
    var inString = false
    var raw = false
    var escaped = false
    var i = 0
    while (i < text.length) {
        val ch = text[i]
        if (inString) {
            if (raw) {
                if (text.startsWith("\"\"\"", i)) {
                    inString = false
                    raw = false
                    i += 2
                }
            } else if (escaped) {
                escaped = false
            } else if (ch == '\\') {
                escaped = true
            } else if (ch == '"') {
                inString = false
            }
            i++
            continue
        }
        when (ch) {
            '"' -> {
                inString = true
                if (text.startsWith("\"\"\"", i)) {
                    raw = true
                    i += 2
                }
            }
            '(' -> depth++
            ')' -> if (depth > 0) depth--
        }
        if (ch != '"' && visit(i, ch, depth)) return
        i++
    }
}

/** Go's sqlStaticOperand. */
fun sqlStaticOperand(operand: String): Boolean {
    var text = operand.trim()
    while (text.startsWith("(") && text.endsWith(")")) {
        val inner = text.substring(1, text.length - 1).trim()
        if (inner.isEmpty()) break
        text = inner
    }
    if (text == "null") return true
    if (text.startsWith("\"")) return !text.contains('$')
    return sqlSchemaConstantName(sqlLastIdentifierSegment(text))
}

// Go's `\$\{?\s*(...)`, with RE2's `\s` ([\t\n\f\r ], no vertical tab).
private val interpolatedName = Regex("""\$\{?[\t\n\u000C\r ]*([A-Za-z_][A-Za-z0-9_.]*)""")

/** Go's sqlInterpolationUsesOnlyStaticSchemaConstants. */
fun sqlInterpolationUsesOnlySchemaConstants(text: String): Boolean {
    val matches = interpolatedName.findAll(text).toList()
    if (matches.isEmpty()) return false
    return matches.all { sqlSchemaConstantName(sqlLastIdentifierSegment(it.groupValues[1])) }
}

/** Go's sqlLastIdentifierSegment. */
fun sqlLastIdentifierSegment(value: String): String {
    var text = value.trim().removeSuffix(")")
    val dot = text.lastIndexOf('.')
    if (dot >= 0) text = text.substring(dot + 1)
    return text.trim('`', ' ')
}

/**
 * Go's sqlSchemaConstantName. Upper-casing is per character, like Go's
 * strings.ToUpper, so a name never changes length.
 */
fun sqlSchemaConstantName(name: String): Boolean {
    if (name.isEmpty()) return false
    if (name.startsWith("TABLE_") || name.startsWith("COLUMN_")) return true
    if (name.endsWith("_TABLE") || name.endsWith("_COLUMN") || name.endsWith("_KEY")) return true
    return name.map { it.uppercaseChar() }.joinToString("") == name && name.contains('_')
}
