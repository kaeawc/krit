package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState

@Composable
fun Clock(): Int {
    val ticks = produceState(0) {
        val current = value
        println(current)
    }
    return ticks.value
}
