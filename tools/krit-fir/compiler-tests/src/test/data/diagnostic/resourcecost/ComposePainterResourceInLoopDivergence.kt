// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 46
// Shapes where the compiler's view differs from Go's name matching.
package test

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.painterResource as loadPainter

object R {
    object drawable {
        const val marker = 1
        const val item = 2
    }
}

// Go matches the written callee name painterResource, so an import alias hides the call from it.
@Composable
fun AliasedImport(rows: List<String>) {
    LazyColumn {
        items(rows) { row ->
            Icon(<!ComposePainterResourceInLoop!>loadPainter<!>(R.drawable.marker), row)
        }
    }
}

// Go treats the identifier `item` in the argument as the loop parameter; here it names a resource constant.
@Composable
fun ConstantNamedLikeParameter(rows: List<String>) {
    LazyColumn {
        items(rows) { item ->
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.item), item)
        }
    }
}

// Go binds only the first name of a destructuring loop variable, so it reports the use of the second.
@Composable
fun ReadsDestructured(rows: List<Pair<String, Int>>) {
    Column {
        for ((name, icon) in rows) {
            Icon(painterResource(icon), name)
        }
    }
}
