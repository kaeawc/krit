package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.State
import androidx.compose.runtime.collectAsState
import kotlinx.coroutines.flow.StateFlow

class Holder { val value = 1 }

@Composable
fun Counter(count: StateFlow<Int>, state: State<Int>, holder: Holder, onClick: (() -> Unit) -> Unit) {
    val current = count.collectAsState().value
    val read = state.value
    val plain = holder.value
    onClick { println(count.value) }
}

fun outsideComposition(count: StateFlow<Int>) = count.value
