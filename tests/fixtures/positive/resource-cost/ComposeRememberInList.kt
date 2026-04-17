package fixtures.positive.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun ComposeRememberInList(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            val state = remember { expensiveBuilder(item) }
            Text(state.toString())
        }
    }
}
