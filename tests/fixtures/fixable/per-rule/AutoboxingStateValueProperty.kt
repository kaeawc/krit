package fixtures.compose

import androidx.compose.runtime.MutableIntState

fun bump(counter: MutableIntState) {
    val current = counter.value
    counter.value = current + 1
}
