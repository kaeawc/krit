package fixtures.compose

import androidx.compose.foundation.text.input.TextFieldBuffer

fun insertNewline(buffer: TextFieldBuffer) {
    buffer.append("\n")
}
