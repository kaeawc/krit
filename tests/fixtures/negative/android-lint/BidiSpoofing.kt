package com.example

/** KDoc is not scanned: ‮ */
// Balanced: ‮text‬ and ⁦isolate⁩
/* A line break ends the segment ‮
   so this comment is fine */
class AccessCheck {
    val balanced = "user‮nimda‬"
    val escaped = "\u202E is only an escape sequence"
    val raw = """first line ‮‬
    second line"""
}
