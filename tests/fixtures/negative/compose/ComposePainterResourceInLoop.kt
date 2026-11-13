// fir-parity: skip the fixture never compiles cleanly: it imports androidx.compose.foundation.lazy.item (a LazyListScope member, not a top-level function) and reads an undeclared item, iconFor and R, so Go stays authoritative for it; the FIR negatives are covered by compiler-tests data
package fixtures.negative.compose

import androidx.compose.foundation.Image
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.item
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun ComposePainterResourceInLoopNegative(items: List<String>) {
    val marker = painterResource(iconFor(item))

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

// A singular LazyListScope.item { } renders exactly once — it is not a loop,
// so calling painterResource() directly inside it is fine.
@Composable
fun ComposePainterResourceInSingularItemNegative() {
    LazyColumn {
        item {
            Image(
                painter = painterResource(R.drawable.header),
                contentDescription = null,
            )
        }
    }
}

// A resource id that depends on the item cannot be hoisted.
@Composable
fun ComposePainterResourceInvariantInLoopNegative(items: List<String>) {
    LazyColumn {
        items(items) { item ->
            Image(
                painter = painterResource(iconFor(item)),
                contentDescription = item,
            )
        }
    }
}
