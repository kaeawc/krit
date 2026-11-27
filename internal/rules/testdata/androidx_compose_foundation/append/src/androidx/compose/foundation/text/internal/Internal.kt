package androidx.compose.foundation.text.internal

import androidx.compose.foundation.text.input.TextFieldBuffer

fun internalAppend(buffer: TextFieldBuffer) {
    buffer.append("x")
    buffer.append('c')
    buffer.append("abc", 0, 1)
    with(buffer) { append("y") }
}

class Holder(val sb: StringBuilder) {
    fun f() = sb.append("x")
}
