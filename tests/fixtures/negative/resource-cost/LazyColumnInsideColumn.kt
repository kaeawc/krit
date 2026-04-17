package fixtures.negative.resourcecost

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable

@Composable
fun LazyColumnInsideColumn(items: List<String>) {
    Column {
        Text("Header")
        LazyColumn {
            items(items) { item ->
                Text(item)
            }
        }
    }
}
