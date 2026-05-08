package fixtures.negative.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun ComposePainterResourceInLoop(items: List<String>) {
    val marker = painterResource(R.drawable.marker)
    LazyColumn {
        items(items) { item ->
            Icon(marker, null)
        }
    }
}
