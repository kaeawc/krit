// Smoke: Color(...) resolves to the top-level factory functions, companion
// colors, copy(), and the value-class payload.
package stubs

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.Shape

val Brand: Color = Color(0xFF6200EE)
val Translucent: Color = Color(0x80FFFFFF)
val FromInt: Color = Color(0x00FF00)
val FromComponents: Color = Color(red = 0.1f, green = 0.2f, blue = 0.3f)
val FromRgb: Color = Color(255, 0, 0)
val Filter: ColorFilter = ColorFilter.tint(Color.Black)
val DefaultShape: Shape = RectangleShape

fun tinted(): Color = Color.Red.copy(alpha = 0.5f)

fun raw(color: Color): ULong = color.value

fun isSet(color: Color): Boolean = color != Color.Unspecified && color.alpha > 0f
