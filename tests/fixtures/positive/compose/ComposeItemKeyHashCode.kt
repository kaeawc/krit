package fixtures.compose

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable

@Composable
fun KeyedList(list: List<String>) {
    LazyColumn {
        items(list, key = { it.hashCode() }) {}
        item(key = list.hashCode()) {}
    }
}
