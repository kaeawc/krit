// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes Go and the checker both leave alone.
package test

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

object R {
    object drawable {
        const val marker = 1
    }
}

data class Row(val icon: Int, val selected: Boolean)

@Composable
fun Hoisted(rows: List<String>) {
    val marker = painterResource(R.drawable.marker)
    LazyColumn {
        items(rows) { row ->
            Icon(marker, row)
        }
    }
}

// The argument reads the explicit lambda parameter.
@Composable
fun ReadsParameter(rows: List<Row>) {
    LazyColumn {
        items(rows) { row ->
            Icon(painterResource(row.icon), null)
        }
    }
}

// The argument reads the implicit `it`.
@Composable
fun ReadsImplicitIt(rows: List<Row>) {
    LazyColumn {
        items(rows) {
            Icon(painterResource(it.icon), null)
        }
    }
}

// The argument reads a local declared in the loop body.
@Composable
fun ReadsLocal(rows: List<Row>) {
    LazyColumn {
        items(rows) { row ->
            val id = row.icon
            Icon(painterResource(id), null)
        }
    }
}

@Composable
fun ReadsForVariable(rows: List<Row>) {
    Column {
        for (row in rows) {
            Icon(painterResource(row.icon), null)
        }
    }
}

// Only some items paint, so hoisting is not free.
@Composable
fun GuardedByIf(rows: List<Row>) {
    LazyColumn {
        items(rows) { row ->
            if (row.selected) {
                Icon(painterResource(R.drawable.marker), null)
            }
        }
    }
}

@Composable
fun GuardedByWhen(rows: List<Row>) {
    LazyColumn {
        items(rows) { row ->
            when {
                row.selected -> Icon(painterResource(R.drawable.marker), null)
                else -> Unit
            }
        }
    }
}

// A slot lambda composes on its own schedule.
@Composable
fun InsideSlot(rows: List<String>) {
    LazyColumn {
        items(rows) { row ->
            Box {
                Icon(painterResource(R.drawable.marker), row)
            }
        }
    }
}

// `item { }` renders once.
@Composable
fun SingleItem() {
    LazyColumn {
        item {
            Icon(painterResource(R.drawable.marker), null)
        }
    }
}

// A few children built by an inline helper outside a lazy list are not worth hoisting.
@Composable
fun InlineHelperOutsideLazyList(rows: List<String>) {
    Column {
        rows.forEach { row ->
            Icon(painterResource(R.drawable.marker), row)
        }
        rows.map { row ->
            Icon(painterResource(R.drawable.marker), row)
        }
        repeat(3) {
            Icon(painterResource(R.drawable.marker), null)
        }
    }
}

@Composable
fun OutsideAnyLoop() {
    Icon(painterResource(R.drawable.marker), null)
}
