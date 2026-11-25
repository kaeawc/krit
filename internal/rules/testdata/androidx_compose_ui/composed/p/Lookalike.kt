package p.lookalike

import androidx.compose.ui.Modifier

fun Modifier.composed(factory: Modifier.() -> Modifier): Modifier = factory()

fun Modifier.local(): Modifier = composed { this }
