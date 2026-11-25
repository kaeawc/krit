package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember

@Composable
fun Counter(show: (Int) -> Unit) {
    val count = remember { mutableIntStateOf(0) }
    show(count.value)
}
