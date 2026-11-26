package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Either(flag: Boolean, content: @Composable () -> Unit) {
    if (flag) content() else content()
}

@Composable
fun Early(flag: Boolean, content: @Composable () -> Unit) {
    if (flag) {
        content()
        return
    }
    content()
}
