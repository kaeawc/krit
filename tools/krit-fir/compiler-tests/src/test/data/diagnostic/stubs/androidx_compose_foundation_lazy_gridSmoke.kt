// Smoke: LazyVerticalGrid with GridCells and keyed grid items.
package stubs

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun GridSmoke(values: List<String>) {
    LazyVerticalGrid(columns = GridCells.Fixed(2), modifier = Modifier.fillMaxSize()) {
        item { Text("header") }
        items(values, key = { it }) { value -> Text(value) }
        numbers()
    }
    LazyVerticalGrid(columns = GridCells.Adaptive(minSize = 128.dp)) {
        items(10) { Text("$it") }
    }
}

fun LazyGridScope.numbers() {
    items(count = 2, key = { it }) { index -> Text("$index") }
}
