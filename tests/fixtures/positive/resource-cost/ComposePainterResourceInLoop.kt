package fixtures.positive.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun ComposePainterResourceInLoop(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            Icon(painterResource(R.drawable.marker), null)
        }
    }
}
