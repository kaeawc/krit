package fixtures.compose

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

@Composable
fun Title(text: String, modifier: Modifier = Modifier, enabled: Boolean = true, content: @Composable () -> Unit) {
    Text(text, modifier)
    content()
}

@Composable
fun Clickable(text: String, modifier: Modifier = Modifier, onClick: (() -> Unit)?) {
    Text(text, modifier)
}

typealias OnClick = () -> Unit

@Composable
fun Aliased(text: String, modifier: Modifier = Modifier, onClick: OnClick) {
    Text(text, modifier)
}

fun notComposable(modifier: Modifier = Modifier, text: String) = text
