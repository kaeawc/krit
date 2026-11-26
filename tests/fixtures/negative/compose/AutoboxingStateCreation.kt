package fixtures.compose

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.neverEqualPolicy

class Settings {
    val fontScale = mutableFloatStateOf(1f)
    val optional: MutableState<Float?> = mutableStateOf(1f)
    val name = mutableStateOf("x")
    val forced = mutableStateOf(1, neverEqualPolicy())
}

object Local {
    fun mutableStateOf(value: Int) = value
    val notState = mutableStateOf(1)
}

fun fixedHelper(block: () -> MutableState<Any>): MutableState<Any> = block()
fun consumeAny(state: MutableState<Any>) {}
fun consumeAll(vararg values: Any) {}

fun notInferred() {
    val viaFixedHelper = fixedHelper { mutableStateOf(1) }
    consumeAny(mutableStateOf(2))
    consumeAll(mutableStateOf(3))
}
