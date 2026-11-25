package p

import androidx.compose.ui.graphics.Color

val missing = Color(0xFF0000)
val missingLower = Color(0xff00ff)
val missingUpperPrefix = Color(0XFF0000)
val missingUnderscores = Color(0xFF_00_00)
val missingLong = Color(0xFF0000L)
val missingNamed = Color(color = 0x00FF00)
val missingFqn = androidx.compose.ui.graphics.Color(0x0000FF)
val missingParens = Color((0x123456))
val complete = Color(0xFF000000)
val completeUnderscores = Color(0xFF_00_00_00)
val invalidSeven = Color(0xFF00000)
val invalidFive = Color(0xFFFFF)
val invalidNine = Color(0xFFFFFFFFF)
val invalidOne = Color(0x0)
val unsigned = Color(0xFF0000u)
val decimal = Color(16711680)
val negative = Color(-0xFF0000)
val channels = Color(0xFF, 0x00, 0x00)
val nonLiteral = Color(missingLong.value.toLong())
val binary = Color(0b111111110000000000000000)

fun inFunction(): Color {
    val local = Color(0xABCDEF)
    return local
}
