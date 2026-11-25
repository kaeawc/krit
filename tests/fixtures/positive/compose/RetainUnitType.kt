package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.retain.retain

class Counter {
    fun reset() {}
}

@Composable
fun Screen(counter: Counter) {
    retain { counter.reset() }
}
