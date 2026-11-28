package com.example

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.State
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp

val LocalSpacing = compositionLocalOf { 4 }

@Composable
fun Greeting(): String = "hi"

@Composable
fun Formatted(name: String): String {
    val upper = name.uppercase()
    println(upper)
    return listOf(upper).joinToString()
}

@Composable
fun Primary(): Color = Color(0xFF000000)

@Composable
fun Padding() = 8.dp

@Composable
fun Themed(): Color = MaterialTheme.colorScheme.primary

@Composable
fun Dark(): Boolean = isSystemInDarkTheme()

@Composable
fun Spacing(): Int = LocalSpacing.current

@Composable
fun Label(): String = stringResource(android.R.string.ok)

@Composable
@ReadOnlyComposable
fun AlreadyReadOnly(): Boolean = isSystemInDarkTheme()

@Composable
fun Emits() {
    Text("x")
}

@Composable
fun Remembers(): Int = remember { 1 }

@Composable
fun ReadsState(state: State<Int>): Int = state.value

@Composable
fun Delegated(state: State<Int>): Int {
    val value by state
    return value
}

@Composable
fun WithSlot(content: @Composable () -> Unit) {
    println("x")
}

@Composable
fun Context(): Any = LocalContext.current

@Composable
fun Helper(): Int = plainHelper()

fun plainHelper(): Int = 1

@Composable
fun CallsComposableHelper(): Boolean = Dark()

@Composable
fun CallsReadOnlyHelper(): Boolean = AlreadyReadOnly()

@Composable
fun DefaultParam(color: Color = MaterialTheme.colorScheme.primary): Color = color

val lambda = @Composable { println("x") }

val composableLambda = @Composable { Text("x") }

val Color.dimmed: Color
    @Composable get() = copy(alpha = 0.5f)

interface Contract {
    @Composable fun render(): Int
}

open class Base {
    @Composable open fun value(): Int = 1
}

@Composable
fun Mutable(): Int {
    val state = remember { mutableStateOf(1) }
    return state.value
}

@Composable
fun MutableStateNoRemember(): Int {
    val state = mutableStateOf(1)
    return state.value
}
