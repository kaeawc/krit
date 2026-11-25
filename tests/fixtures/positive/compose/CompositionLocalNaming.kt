package fixtures.compose

import androidx.compose.runtime.CompositionLocal
import androidx.compose.runtime.compositionLocalOf

val ThemeCompositionLocal = compositionLocalOf { 0 }

object Locals {
    val Spacing: CompositionLocal<Int> = compositionLocalOf { 8 }
}
