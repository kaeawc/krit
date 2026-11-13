package fixtures.positive.compose

import androidx.compose.foundation.Image
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

// The resource ids live in generated code in a real project.
private object R {
    object drawable {
        const val marker = 1
    }
}

@Composable
fun ComposePainterResourceInLoopPositive(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            Image(
                painter = painterResource(R.drawable.marker),
                contentDescription = item,
            )
        }
    }

    items.forEach { item ->
        Image(
            painter = painterResource(R.drawable.marker),
            contentDescription = item,
        )
    }
}
