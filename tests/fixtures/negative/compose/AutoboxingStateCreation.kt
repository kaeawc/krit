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
