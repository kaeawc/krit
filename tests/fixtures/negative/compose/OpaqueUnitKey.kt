package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember

fun refresh() {}
fun revision(): Int = 1

@Composable
fun Screen(id: String) {
    refresh()
    LaunchedEffect(Unit) { }
    LaunchedEffect(id) { }
    val cached = remember(revision()) { id.length }
}
