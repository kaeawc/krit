package androidx.compose.foundation.text.input.internal

import androidx.compose.foundation.text.input.TextFieldBuffer

internal fun insertNewline(buffer: TextFieldBuffer) {
    buffer.append("\n")
}
