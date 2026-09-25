// Compiler-test source stubs; never packaged in the production artifact.
package androidx.activity.compose

import androidx.activity.ComponentActivity
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionContext

fun ComponentActivity.setContent(
    parent: CompositionContext? = null,
    content: @Composable () -> Unit,
) {
    TODO()
}

@Composable
fun BackHandler(enabled: Boolean = true, onBack: () -> Unit) {
    TODO()
}
