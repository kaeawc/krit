package q

import androidx.compose.runtime.*

@Composable
fun starLower() {}

@androidx.compose.runtime.Composable
fun fqnLower() {}

val StarLocalName = staticCompositionLocalOf<Int> { 1 }

val Getter: Int
    @Composable get() = 1

@Composable
fun Wrapped(): String = wrappedValue()

@Composable
fun wrappedValue(): String = ""

@Composable
fun Unannotated(content: () -> Unit, @Suppress("x") text: @Composable () -> Unit, more: Int) {}

/* @Composable fun commentLower() {} */
val s = "@Composable fun stringLower() {}"
