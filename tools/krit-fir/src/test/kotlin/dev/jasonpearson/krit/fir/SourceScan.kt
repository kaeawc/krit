package dev.jasonpearson.krit.fir

/**
 * Removes comments (line, and block comments which nest and leave a space) and blanks the contents of string
 * literals (plain and raw) and char literals, keeping the quotes and every newline so line structure survives. Code
 * inside a string template (`${...}` and `$name`) is kept, and literals inside it are blanked too. Used by the tests
 * that scan checker sources on disk.
 */
internal fun stripCommentsAndStrings(src: String): String = SourceStripper(src).strip()

private class SourceStripper(private val src: String) {
    private val out = StringBuilder()
    private var i = 0

    fun strip(): String {
        code(insideTemplate = false)
        return out.toString()
    }

    /** Copies code up to the end of the input or, inside a template, up to the brace that closes it. */
    private fun code(insideTemplate: Boolean) {
        var braces = 0
        while (i < src.length) {
            val c = src[i]
            when {
                src.startsWith("//", i) -> skipLineComment()
                src.startsWith("/*", i) -> skipBlockComment()
                src.startsWith("\"\"\"", i) -> string(RAW_QUOTE)
                c == '"' -> string(PLAIN_QUOTE)
                c == '\'' -> charLiteral()
                insideTemplate && c == '}' && braces == 0 -> return
                else -> {
                    braces += braceDelta(c)
                    out.append(c)
                    i++
                }
            }
        }
    }

    private fun braceDelta(c: Char): Int = when (c) {
        '{' -> 1
        '}' -> -1
        else -> 0
    }

    private fun skipLineComment() {
        while (i < src.length && src[i] != '\n') i++
    }

    private fun skipBlockComment() {
        // A comment separates tokens, so it leaves a space behind.
        out.append(' ')
        var depth = 1
        i += 2
        while (i < src.length && depth > 0) {
            when {
                src.startsWith("/*", i) -> {
                    depth++
                    i += 2
                }
                src.startsWith("*/", i) -> {
                    depth--
                    i += 2
                }
                else -> blank()
            }
        }
    }

    private fun string(quote: String) {
        val raw = quote == RAW_QUOTE
        out.append(quote)
        i += quote.length
        while (i < src.length && !src.startsWith(quote, i)) {
            when {
                !raw && src[i] == '\\' -> i += 2
                src.startsWith("\${", i) -> template()
                src[i] == '$' && src.getOrNull(i + 1)?.isIdentifierStart() == true -> simpleTemplate()
                else -> blank()
            }
        }
        // A raw string may end with extra quotes ("""" is one quote of content plus the close).
        while (raw && src.startsWith("\"\"\"\"", i)) i++
        if (i < src.length) {
            out.append(quote)
            i += quote.length
        }
    }

    /** Copies a `$name` template: the name is code. */
    private fun simpleTemplate() {
        val end = (i + 1 until src.length).firstOrNull { !src[it].isLetterOrDigit() && src[it] != '_' } ?: src.length
        out.append(src, i, end)
        i = end
    }

    private fun Char.isIdentifierStart(): Boolean = isLetter() || this == '_'

    private fun template() {
        out.append("\${")
        i += 2
        code(insideTemplate = true)
        if (i < src.length) {
            out.append('}')
            i++
        }
    }

    /** A char literal is 'x' or '\x...'; an apostrophe that is not one is copied as code. */
    private fun charLiteral() {
        val close = when {
            src.startsWith("\\", i + 1) -> src.indexOf('\'', i + 3)
            src.startsWith("'", i + 2) -> i + 2
            else -> -1
        }
        if (close in (i + 1)..(i + MAX_CHAR_LITERAL_LENGTH)) {
            out.append("'x'")
            i = close + 1
        } else {
            out.append('\'')
            i++
        }
    }

    /** Drops the character at [i], keeping a newline so line numbers do not move. */
    private fun blank() {
        if (src[i] == '\n') out.append('\n')
        i++
    }
}

private const val PLAIN_QUOTE = "\""
private const val RAW_QUOTE = "\"\"\""

// Wider than the longest char literal, a unicode escape ('\uXXXX').
private const val MAX_CHAR_LITERAL_LENGTH = 12
