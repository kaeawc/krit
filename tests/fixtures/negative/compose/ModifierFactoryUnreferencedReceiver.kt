package fixtures.compose

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Modifier.explicit(): Modifier = this.padding(8.dp)

fun Modifier.implicit(): Modifier = padding(8.dp)

fun Modifier.chained(): Modifier = then(Modifier.padding(8.dp))

fun Modifier.labeled(): Modifier = run { this@labeled.padding(8.dp) }

abstract class Factories {
    abstract fun Modifier.declared(): Modifier
}
