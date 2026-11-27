package androidx.compose.foundation.text.internal

import androidx.compose.foundation.text.input.TextFieldState
import androidx.compose.ui.text.buildAnnotatedString

fun builders(state: TextFieldState) {
    state.edit { buildAnnotatedString { append("x") } }
    state.edit { buildString { append("y") } }
    state.edit { append("z") }
}
