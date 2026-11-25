package r

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.mutableStateSetOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.snapshots.StateFactoryMarker

@Composable
fun Locals() {
    val legacy = mutableStateOf(0)
    var delegated by mutableStateOf(0)
    Use(mutableStateListOf<Int>())
    Use(mutableStateMapOf<Int, Float>())
    Use(mutableStateSetOf<Int>())
    Use(mutableIntStateOf(1))
    Use(derivedStateOf { legacy.value })
    val ok = remember { mutableStateOf(0) }
    val ok2 = remember(legacy) { derivedStateOf { legacy.value } }
    val saved = rememberSaveable { mutableStateOf(0) }
    LaunchedEffect(Unit) { mutableStateOf(1) }
    val onClick = { mutableStateOf(1) }
    Use(makeMyState())
    Use(CustomState<Nothing>())
    val anon = object : MutableState<Int> {
        override var value: Int = 0
        override fun component1(): Int = value
        override fun component2(): (Int) -> Unit = {}
    }
}

@Composable
fun Lambda(content: @Composable () -> Unit) {
    Lambda { Use(mutableStateOf(0)) }
    1.let { Use(mutableStateOf(it)) }
}

val topLevel = @Composable { Use(mutableStateOf(0)) }

fun notComposable() {
    val s = mutableStateOf(0)
}

fun Use(any: Any?) {}

@StateFactoryMarker
fun makeMyState(): State<Int> = mutableStateOf(1)

class CustomState<T> : State<T> {
    override val value: T get() = throw Exception()
}

class Holder {
    val s = mutableStateOf(0)
}
