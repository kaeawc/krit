package fixtures.compose

import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.compositionLocalOf

var LocalMutable = compositionLocalOf { 0 }

val LocalQualified = androidx.compose.runtime.compositionLocalOf { 0 }

fun build() {
    val LocalInFunction = compositionLocalOf { 1 }
    println(LocalInFunction)
}

object Other {
    fun compositionLocalOf(block: () -> Int) = block()
    val NotALocal = compositionLocalOf { 1 }
}

val text = "val LocalText = compositionLocalOf { 1 }"
