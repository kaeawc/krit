package stubs

import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusState
import androidx.compose.ui.focus.onFocusChanged

fun FocusSmoke(modifier: Modifier, callback: (FocusState) -> Unit): Modifier = modifier.onFocusChanged(callback)
