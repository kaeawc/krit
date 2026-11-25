package p

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun Good(text: String, modifier: Modifier = Modifier, enabled: Boolean = true) {
    Box(modifier)
}

@Composable
fun GoodRequired(modifier: Modifier) {
    Box(modifier)
}

@Composable
fun WrongName(text: String, mod: Modifier = Modifier) {
    Box(mod)
}

@Composable
fun WrongType(modifier: Modifier.Element) {
}

@Composable
fun WrongDefault(modifier: Modifier = Modifier.padding(1.dp)) {
    Box(modifier)
}

@Composable
fun NotFirstOptional(enabled: Boolean = true, modifier: Modifier = Modifier) {
    Box(modifier)
}

@Composable
fun Everything(enabled: Boolean = true, m: Modifier.Element = Modifier.Element) {
}

@Composable
fun NullableModifier(modifier: Modifier? = null) {
}

@Composable
fun TwoModifiers(first: Modifier = Modifier, modifier: Modifier = Modifier) {
}

@Composable
fun ReturnsValue(mod: Modifier = Modifier): Int = 1

@Composable
fun ExplicitUnit(mod: Modifier): Unit {
}

@Composable
fun ExpressionBody(mod: Modifier) = Box(mod)

fun NotComposable(mod: Modifier) {
}

class Holder {
    @Composable
    fun Member(mod: Modifier) {
    }
}

@Composable
fun LambdaParam(content: @Composable (Modifier) -> Unit) {
}

@Composable
fun Outer() {
    @Composable
    fun Local(mod: Modifier) {
    }
}

@Composable
fun FqnDefault(modifier: Modifier = androidx.compose.ui.Modifier) {
}

@Composable
fun Trailing(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
}
