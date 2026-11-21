package androidx.compose.ui.focus

import androidx.compose.ui.Modifier

interface FocusState {
    val hasFocus: Boolean
    val isFocused: Boolean
    val isCaptured: Boolean
}

fun Modifier.onFocusChanged(onFocusChanged: (FocusState) -> Unit): Modifier = TODO()
