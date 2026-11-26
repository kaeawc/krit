package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.NonRestartableComposable

@NonRestartableComposable
@Composable
fun Label(text: String) {
    Text(text)
}

@Composable
fun Shouting(text: String) {
    Text(text.uppercase())
}

@Composable
fun Twice(text: String) {
    Text(text)
    Text(text)
}
