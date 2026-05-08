package fixtures.negative.compose

import androidx.compose.foundation.Image
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun ComposePainterResourceInLoopNegative(items: List<String>) {
    val marker = painterResource(R.drawable.marker)

    LazyColumn {
        items(items) { item ->
            Image(
                painter = marker,
                contentDescription = item,
            )
        }
    }

    items.forEach { item ->
        Image(
            painter = marker,
            contentDescription = item,
        )
    }
}
