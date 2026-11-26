package fixtures.compose

import androidx.compose.runtime.compositionLocalOf

val LocalTheme = compositionLocalOf { 0 }

val LocalComputed: Any
    get() {
        println("not first")
        return compositionLocalOf { 1 }
    }

object Other {
    fun compositionLocalOf(block: () -> Int) = block()
    val NotALocal get() = compositionLocalOf { 1 }
}

val Unrelated get() = listOf(1)
