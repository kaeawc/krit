package fixtures.positive.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.ui.viewinterop.AndroidView

@Composable
fun RecyclerViewInLazyColumn(items: List<String>) {
    LazyColumn {
        item {
            AndroidView(factory = { context ->
                androidx.recyclerview.widget.RecyclerView(context)
            })
        }
    }
}
