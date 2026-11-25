package fixtures.compose

import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

fun Modifier.cardPadding(): Modifier = this.then(padding(8.dp))
