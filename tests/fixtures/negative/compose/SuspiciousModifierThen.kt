package fixtures.compose

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Modifier.cardPadding(): Modifier = this.then(Modifier.padding(8.dp))

fun Modifier.scoped(): Modifier = then(with(Modifier) { padding(8.dp) })

class Chain {
    fun then(other: Chain): Chain = other
    fun next(): Chain = this
    fun build(): Chain = then(next())
}
