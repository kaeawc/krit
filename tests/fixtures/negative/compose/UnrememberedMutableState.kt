package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue

class TodoViewModel {
    val items = mutableStateListOf<String>()
}

@Composable
fun TodoList(onClick: (() -> Unit) -> Unit) {
    val items = remember { mutableStateListOf<String>() }
    val count = remember(items) { mutableStateOf(items.size) }
    LaunchedEffect(Unit) { mutableStateOf(1) }
    onClick { mutableStateOf(2) }
}

fun mutableStateListOfLookalike() = Local.mutableStateListOf()

object Local {
    fun mutableStateListOf(): List<String> = emptyList()
}

@Composable
fun DelegatedCounter(show: (Int) -> Unit) {
    var count by remember { mutableStateOf(0) }
    show(count)
}
