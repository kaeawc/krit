package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableIntState
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember

class Box(val value: Int)

@Composable
fun Counter(show: (Int) -> Unit, generic: State<Int>, box: Box, counter: MutableIntState) {
    val count = remember { mutableIntStateOf(0) }
    show(count.intValue)
    show(generic.value)
    show(box.value)
    counter.value += 1
}
