package fixtures.compose

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Modifier.cardPadding(): Modifier = padding(8.dp)

fun outer() {
    fun localModifier(): Modifier = Modifier
}

val cardModifier: Modifier = Modifier.padding(8.dp)

fun cardWidth(): Int = 8
