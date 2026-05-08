package fixtures.negative.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun ComposeRememberInList(items: List<String>) {
    LazyColumn {
        items(items, key = { it }) { item ->
            val state = remember(item) { expensiveBuilder(item) }
            Text(state.toString())
        }
    }
}
