package fixtures.compose

import androidx.compose.material.lightColors
import androidx.compose.ui.graphics.Color

val Brand = Color(0xFF6200EE)

val AppColors = lightColors(
    primary = Brand,
    primaryVariant = Brand,
    background = Color.White,
    surface = Color.White,
    onPrimary = Color.White,
    onBackground = Color.Black,
    onSurface = Color.Black,
)

fun lightColors(primary: Color, onPrimary: Color, background: Color, onBackground: Color) = Unit

val NotMaterial = fixtures.compose.lightColors(Color.White, Color.Black, Color.White, Color.Red)
