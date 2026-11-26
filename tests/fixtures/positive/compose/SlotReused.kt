package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Twice(content: @Composable () -> Unit) {
    content()
    content()
}
