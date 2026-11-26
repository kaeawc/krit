package com.example

@Suppress("TrimLambda")
fun clean(s: String, cs: CharSequence, maybe: String?): List<Any?> = listOf(
    s.trim { it <= ' ' },
    s.trim() { it <= ' ' },
    s.trim({ it <= ' ' }),
    cs.trim { c -> c <= ' ' },
    maybe?.trim { it.isWhitespace() },
    "  x ".trim { (it <= ' ') },
    s.trim { ch -> ch.isWhitespace() },
)
