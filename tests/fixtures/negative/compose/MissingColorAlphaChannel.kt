package fixtures.compose

import androidx.compose.ui.graphics.Color

val Opaque = Color(0xFF3366FF)
val Channels = Color(0x33, 0x66, 0xFF)
val Parenthesized = Color((0x3366FF))

class Swatch(val value: Long)

fun Swatch(value: Int): Swatch = Swatch(value.toLong())

val Other = Swatch(0x3366FF)
