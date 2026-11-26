package com.example

class Box {
    fun trim(predicate: (Char) -> Boolean): Box = this
}

fun clean(s: String, box: Box): List<Any?> = listOf(
    s.trim(),
    s.trim { it == ' ' },
    s.trim { it < ' ' },
    s.trim { it <= '\t' },
    s.trim { ' ' >= it },
    s.trim { c -> c.isDigit() },
    s.trim { it.isWhitespace() && it != '\n' },
    s.trim { other(it) },
    s.trimStart { it <= ' ' },
    box.trim { it <= ' ' },
    s.trim(Char::isWhitespace),
    // s.trim { it <= ' ' }
    "s.trim { it <= ' ' }",
)

fun other(c: Char) = c <= ' '
