package fixtures.compose

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key

@Composable
fun StableKeys(list: List<String>) {
    LazyColumn {
        items(list, key = { it }) {}
        items(list, key = { it.hashCode(1) }) {}
    }
    key(list.hashCode()) {}
    val label = "key = { it.hashCode() }"
    println(label)
}

fun LazyListScope.notComposable(list: List<String>) {
    items(list, key = { it.hashCode() }) {}
}

fun String.hashCode(seed: Int): Int = seed
