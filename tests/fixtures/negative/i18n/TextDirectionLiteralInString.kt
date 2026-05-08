package test

import androidx.core.text.BidiFormatter

fun wrapLtr(s: String): String = BidiFormatter.getInstance().unicodeWrap(s)

fun escapedBackslash(): String = "\\u200E is the LRM codepoint."

fun unrelatedUnicode(): String = "snowman ☃ with id é"
