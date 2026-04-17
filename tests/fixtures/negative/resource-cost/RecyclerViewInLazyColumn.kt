package fixtures.negative.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable

@Composable
fun RecyclerViewInLazyColumn(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            Text(item)
        }
    }
}
