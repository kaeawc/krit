package fixtures.compose

import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.staticCompositionLocalOf

val LocalTheme get() = compositionLocalOf { 0 }

val LocalSpacing: Any
    get() {
        return (staticCompositionLocalOf { 8 })
    }
