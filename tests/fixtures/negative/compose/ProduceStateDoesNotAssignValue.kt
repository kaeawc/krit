package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import kotlinx.coroutines.delay

@Composable
fun Clock(load: suspend () -> Int): Int {
    val ticks = produceState(0) {
        while (true) {
            delay(1000)
            value++
        }
    }
    val loaded = produceState(0, load) { value = load() }
    return ticks.value + loaded.value
}
