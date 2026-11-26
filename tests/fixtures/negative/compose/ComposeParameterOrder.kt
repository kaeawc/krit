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

// kotlin.Function*, kotlin.jvm.functions.* and the auto-imported Comparator are trailing functions too.
@Composable
fun Block(text: String, modifier: Modifier = Modifier, block: Function0<Unit>) {
    Text(text, modifier)
}

@Composable
fun JvmBlock(text: String, modifier: Modifier = Modifier, block: kotlin.jvm.functions.Function1<Int, Unit>) {
    Text(text, modifier)
}

@Composable
fun Sorted(text: String, modifier: Modifier = Modifier, cmp: Comparator<String>) {
    Text(text, modifier)
}
