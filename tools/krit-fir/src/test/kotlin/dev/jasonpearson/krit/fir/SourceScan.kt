package dev.jasonpearson.krit.fir

/**
 * Removes comments (line, and block comments which nest) and blanks the contents of string literals (plain and
 * raw) and char literals, keeping the quotes and every newline so line structure survives. Used by the tests that
 * scan checker sources on disk.
 */
internal fun stripCommentsAndStrings(src: String): String {
    val out = StringBuilder()
    var i = 0
    val n = src.length
    while (i < n) {
        val c = src[i]
        when {
            src.startsWith("//", i) -> while (i < n && src[i] != '\n') i++
            src.startsWith("/*", i) -> {
                var depth = 1
                i += 2
                while (i < n && depth > 0) {
                    when {
                        src.startsWith("/*", i) -> { depth++; i += 2 }
                        src.startsWith("*/", i) -> { depth--; i += 2 }
                        else -> { if (src[i] == '\n') out.append('\n'); i++ }
                    }
                }
            }
            src.startsWith("\"\"\"", i) -> {
                out.append("\"\"\"")
                i += 3
                while (i < n && !src.startsWith("\"\"\"", i)) { if (src[i] == '\n') out.append('\n'); i++ }
                // A raw string may end with extra quotes ("""" is one quote of content plus the close).
                while (i < n && src.startsWith("\"\"\"\"", i)) i++
                if (i < n) { out.append("\"\"\""); i += 3 }
            }
            c == '"' -> {
                out.append('"')
                i++
                while (i < n && src[i] != '"') { if (src[i] == '\\') i++; i++ }
                out.append('"')
                i++
            }
            c == '\'' -> {
                // A char literal is 'x' or '\x...'; an apostrophe that is not one is left alone.
                val close = if (i + 1 < n && src[i + 1] == '\\') src.indexOf('\'', i + 3) else if (i + 2 < n && src[i + 2] == '\'') i + 2 else -1
                if (close in (i + 1)..(i + 12)) { out.append('\'').append('x').append('\''); i = close + 1 } else { out.append(c); i++ }
            }
            else -> { out.append(c); i++ }
        }
    }
    return out.toString()
}
