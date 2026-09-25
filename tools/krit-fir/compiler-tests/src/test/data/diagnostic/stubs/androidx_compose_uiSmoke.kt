// Smoke: Modifier parameter default, chaining with then(), Alignment constants.
package stubs

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun ModifierSmoke(modifier: Modifier = Modifier) {
    val combined: Modifier = modifier.then(Modifier.padding(4.dp))
    val companion: Modifier.Companion = Modifier
    val horizontal: Alignment.Horizontal = Alignment.End
    val vertical: Alignment.Vertical = Alignment.Bottom
    Box(modifier = combined, contentAlignment = Alignment.TopStart) {}
    println("$companion $horizontal $vertical")
}
