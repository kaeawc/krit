package fixtures.compose

import androidx.compose.runtime.CompositionLocal
import androidx.compose.runtime.compositionLocalOf

val LocalTheme = compositionLocalOf { 0 }
val localSpacing = compositionLocalOf { 8 }

class Holder(val Provided: CompositionLocal<Int>)

fun build(param: CompositionLocal<Int>) {
    val inner = compositionLocalOf { 1 }
}

object Other {
    fun compositionLocalOf(block: () -> Int) = block()
    val NotALocal = compositionLocalOf { 1 }
}
