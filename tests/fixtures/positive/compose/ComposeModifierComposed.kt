package fixtures.compose

import androidx.compose.ui.Modifier
import androidx.compose.ui.composed

fun Modifier.highlighted(): Modifier = composed { this }

val Ripple = Modifier.composed { this }

fun Modifier.wrapped(modifier: Modifier): Modifier = modifier.composed { this }
