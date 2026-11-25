package fixtures.compose

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Card(title: String, mod: Modifier = Modifier) {
    Box(mod)
}
