package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.retain.retain

class Counter {
    fun snapshot(): Int = 0
}

@Composable
fun Screen(counter: Counter) {
    val value = retain { counter.snapshot() }
    val holder = retain { Counter() }
}
