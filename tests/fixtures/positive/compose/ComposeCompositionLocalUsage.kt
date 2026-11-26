package fixtures.compose

import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf

val LocalTheme = compositionLocalOf { 0 }

private val LocalSpacing = staticCompositionLocalOf<Int> { 8 }

object Tokens {
    val LocalElevation = (compositionLocalOf { 1 })
}
