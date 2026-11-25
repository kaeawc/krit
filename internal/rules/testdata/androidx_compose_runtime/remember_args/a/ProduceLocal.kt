package a

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState

@Composable
fun ProduceLocal() {
    produceState(0) {
        val current = value
        println(current)
    }
}
