package p

import androidx.compose.ui.Modifier
import androidx.compose.ui.composed

fun Modifier.crossFileHelper(): Modifier = composed { otherHelper() }
fun Modifier.crossFileComposable(): Modifier = composed { otherComposable() }
