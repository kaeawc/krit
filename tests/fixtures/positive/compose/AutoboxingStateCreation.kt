package fixtures.compose

import androidx.compose.runtime.mutableStateOf

class Settings {
    val fontScale = mutableStateOf(1f)
}

inline fun <T> rememberValue(block: () -> T): T = block()
fun consume(value: Any?) {}

fun inferred() {
    val viaHelper = rememberValue { mutableStateOf(1) }
    consume(mutableStateOf(2))
}
