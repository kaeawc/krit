// Smoke: LazyColumn/LazyRow DSL with keyed items, itemsIndexed, and item scopes.
package stubs

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

data class LazyEntry(val id: Long, val name: String)

@Composable
fun EntryList(entries: List<LazyEntry>, onClick: (LazyEntry) -> Unit) {
    val state = rememberLazyListState()
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        state = state,
        contentPadding = PaddingValues(8.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        item(key = "header") { Text("Header") }
        items(entries, key = { it.id }) { entry ->
            Text(entry.name, modifier = Modifier.fillParentMaxWidth().clickable { onClick(entry) })
        }
        itemsIndexed(entries, key = { _, entry -> entry.id }) { index, entry -> Text("$index ${entry.name}") }
        items(count = 3) { index -> Text("$index") }
        footer()
    }
    LazyRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        items(entries) { Text(it.name) }
    }
    println(state.firstVisibleItemIndex)
}

fun LazyListScope.footer() {
    item { Text("Footer") }
}
