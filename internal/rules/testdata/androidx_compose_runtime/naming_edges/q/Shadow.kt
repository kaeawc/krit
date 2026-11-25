package q

import androidx.compose.runtime.compositionLocalOf

object Other {
    fun compositionLocalOf(block: () -> Int) = block()
    val NotALocal = compositionLocalOf { 1 }
}

class Real {
    val RealCompositionLocal = compositionLocalOf { 1 }
}
