package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Labels(modifier: Modifier = Modifier) {
    Text("a", modifier = modifier)
    Text("b", modifier = modifier)
}
