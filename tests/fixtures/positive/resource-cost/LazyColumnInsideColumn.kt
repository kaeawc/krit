package fixtures.positive.resourcecost

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable

@Composable
fun LazyColumnInsideColumn(items: List<String>) {
    Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
        Text("Header")
        LazyColumn {
            items(items) { item ->
                Text(item)
            }
        }
    }
}
