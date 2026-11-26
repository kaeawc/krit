package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Title(modifier: Modifier = Modifier, text: String) {
    Text(text, modifier)
}

@Composable
fun Subtitle(text: String, enabled: Boolean = true, modifier: Modifier = Modifier) {
    Text(text, modifier)
}
