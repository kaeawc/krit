package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf

@Composable
fun TodoList(onItems: (List<String>) -> Unit) {
    val items = mutableStateListOf<String>()
    onItems(items)
    Counter(mutableStateOf(0))
}

@Composable
fun Counter(count: androidx.compose.runtime.MutableState<Int>) {}

// Formerly reported by the retired ComposeMutableStateInComposition rule.
@Composable
fun PlainLocalCounter(show: (Int) -> Unit) {
    val count = mutableStateOf(0)
    show(count.value)
}
