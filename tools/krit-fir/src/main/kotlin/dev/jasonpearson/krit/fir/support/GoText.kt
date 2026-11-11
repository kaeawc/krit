package dev.jasonpearson.krit.fir.support

/**
 * Go's `strings.TrimSpace`: trims runes where `unicode.IsSpace` holds.
 *
 * Kotlin's `trim()` differs: it trims U+001C..U+001F and keeps U+0085.
 */
fun trimGoSpace(text: String): String = text.trim(::isGoSpace)

/** Go's `unicode.IsSpace` for one UTF-16 unit: Latin-1 spaces, U+0085, U+00A0 and White_Space above Latin-1. */
fun isGoSpace(c: Char): Boolean = when (c) {
    '\t', '\n', '\u000B', '\u000C', '\r', ' ', '\u0085', ' ',
    ' ', ' ', ' ', ' ', ' ', '　' -> true
    else -> c in ' '..' '
}

/**
 * Go's `strings.ToUpper`: maps each code point to its simple uppercase form.
 *
 * `String.uppercase()` applies full mappings, which can turn one char into several.
 */
fun upperCodePoints(text: String): String {
    val out = StringBuilder(text.length)
    var i = 0
    while (i < text.length) {
        val cp = text.codePointAt(i)
        out.appendCodePoint(Character.toUpperCase(cp))
        i += Character.charCount(cp)
    }
    return out.toString()
}
