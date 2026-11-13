// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 34, 43, 52, 63, 75, 76, 87
// painterResource() whose result is the same every iteration of a list or loop body.
package test

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

object R {
    object drawable {
        const val marker = 1
        const val star = 2
    }
}

@Composable
fun ItemsLoop(rows: List<String>) {
    LazyColumn {
        items(rows) { row ->
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.marker), row)
        }
    }
}

@Composable
fun ItemsIndexedLoop(rows: List<String>) {
    LazyColumn {
        itemsIndexed(rows) { index, row ->
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.marker), "$index $row")
        }
    }
}

@Composable
fun ItemsCountLoop() {
    LazyColumn {
        items(10) {
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.marker), null)
        }
    }
}

@Composable
fun ForLoop(rows: List<String>) {
    Column {
        for (row in rows) {
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.marker), row)
        }
    }
}

// An inline helper inside a lazy list lambda still runs per item.
@Composable
fun ForEachInsideItems(rows: List<List<String>>) {
    LazyColumn {
        items(rows) { row ->
            row.forEach { cell ->
                Icon(<!ComposePainterResourceInLoop!>painterResource<!>(R.drawable.marker), cell)
            }
        }
    }
}

// The argument names a value declared outside the loop, so it can be hoisted.
@Composable
fun OuterValue(rows: List<String>, icon: Int) {
    val star = R.drawable.star
    LazyColumn {
        items(rows) { row ->
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(icon), row)
            Icon(<!ComposePainterResourceInLoop!>painterResource<!>(star), row)
        }
    }
}

// A call arranged over several lines reports on its first line.
@Composable
fun MultiLine(rows: List<String>) {
    LazyColumn {
        items(rows) {
            Icon(
                <!ComposePainterResourceInLoop!>painterResource<!>(
                    R.drawable.marker,
                ),
                null,
            )
        }
    }
}
