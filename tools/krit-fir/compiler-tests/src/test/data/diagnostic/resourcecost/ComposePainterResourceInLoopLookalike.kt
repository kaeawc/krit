// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 41
// Names that only look like the Compose painter loader or a loop call.
package test

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.painter.Painter

object R {
    object drawable {
        const val marker = 1
    }
}

// Go matches the written name, so it reports the call below; it does not load a Compose painter.
fun painterResource(id: Int): Painter = TODO()

@Composable
fun LookalikeLoader(rows: List<String>) {
    LazyColumn {
        items(rows) { row ->
            Icon(painterResource(R.drawable.marker), row)
        }
    }
}

class Pager {
    fun items(count: Int, content: (Int) -> Unit) = content(count)
}

fun repeat(count: Int, content: (Int) -> Unit) = content(count)

// Go reports the first call: `items` here is a plain method, not a lazy list builder. The `repeat` call runs
// outside a lazy list and none of the calls is reported by the checker.
@Composable
fun LookalikeLoops(pager: Pager) {
    pager.items(3) {
        Icon(androidx.compose.ui.res.painterResource(R.drawable.marker), null)
    }
    repeat(3) {
        Icon(androidx.compose.ui.res.painterResource(R.drawable.marker), null)
    }
}
