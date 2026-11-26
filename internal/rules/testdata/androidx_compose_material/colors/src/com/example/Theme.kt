package com.example

import androidx.compose.material.Colors
import androidx.compose.material.darkColors
import androidx.compose.material.lightColors
import androidx.compose.ui.graphics.Color

val Purple = Color(0xFF6200EE)
val AlsoPurple = Purple
var Mutable = Color.Red

val conflictingLight = lightColors(
    primary = Color.White,
    background = Color.White,
    onPrimary = Color.Black,
    onBackground = Color.Red,
)

val sameOnColors = lightColors(
    primary = Color.White,
    background = Color.White,
    onPrimary = Color.Black,
    onBackground = Color.Black,
)

val defaultOnColor = darkColors(
    surface = Color.Blue,
    background = Color.Blue,
    onSurface = Color.Yellow,
)

val throughVals = lightColors(
    primary = Purple,
    secondary = AlsoPurple,
    onPrimary = Color.White,
    onSecondary = Color.Black,
)

val variants = lightColors(
    primary = Color.Green,
    primaryVariant = Color.Green,
    onPrimary = Color.White,
)

val textDiffers = lightColors(
    primary = Color(0xFFFFFFFF),
    error = Color(0xFFFFFFFF),
    onPrimary = Color(0xFF000000),
    onError = Color( 0xFF000000 ),
)

fun local(): Colors {
    val bg = Color.Cyan
    var on = Color.Black
    on = Color.Magenta
    return lightColors(
        surface = bg,
        background = Color.Cyan,
        onSurface = on,
        onBackground = Color.Black,
    )
}

fun param(bg: Color, onBg: Color) = lightColors(
    surface = bg,
    background = bg,
    onSurface = onBg,
    onBackground = Color.Black,
)

val mutableRef = lightColors(
    surface = Mutable,
    background = Color.Red,
    onSurface = Color.Black,
    onBackground = Color.White,
)

val ctor = Colors(
    Color.White, Color.White, Color.Gray, Color.Gray, Color.White, Color.White, Color.Red,
    Color.Black, Color.Black, Color.Blue, Color.Black, Color.White, true,
)

val ctorNamed = Colors(
    primary = Color.White,
    primaryVariant = Color.Gray,
    secondary = Color.Gray,
    secondaryVariant = Color.Gray,
    background = Color.Gray,
    surface = Color.Gray,
    error = Color.Red,
    onPrimary = Color.Black,
    onSecondary = Color.Black,
    onBackground = Color.Blue,
    onSurface = Color.Blue,
    onError = Color.White,
    isLight = true,
)

val positional = lightColors(Color.White, Color.White, Color.White, Color.White, Color.White, Color.White, Color.White,
    Color.Black, Color.Red)
