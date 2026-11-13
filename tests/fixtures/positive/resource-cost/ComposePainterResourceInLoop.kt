package fixtures.positive.resourcecost

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

// The resource ids live in generated code in a real project.
private object R {
    object drawable {
        const val marker = 1
    }
}

@Composable
fun ComposePainterResourceInLoop(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            Icon(painterResource(R.drawable.marker), null)
        }
    }
}
