// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 53
// Keyless remember calls whose staleness is already handled: a parameter covered by an enclosing key(...) block,
// the keyless arm of an if whose other arm is keyed, a wrapped lambda, and a Circuit Navigator.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.runtime.movableContentOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState

interface Navigator

fun use(x: Any?) {}

@Composable
fun KeyedByBlock(id: Int) {
    key(id) {
        val v = remember { id + 1 }
        use(v)
    }
}

@Composable
fun FallbackArm(token: String?) {
    val v = if (token != null) remember(token) { token.length } else remember { token.hashCode() }
    use(v)
}

@Composable
fun MovableWrapper(content: @Composable () -> Unit) {
    val movable = remember { movableContentOf { content() } }
    use(movable)
}

@Composable
fun UpdatedWrapper(callback: () -> Unit) {
    val state = remember { rememberUpdatedState(callback) }
    use(state)
}

@Composable
fun NavigatorCapture(navigator: Navigator) {
    val v = remember { navigator.toString() }
    use(v)
}

// The block keys only `id`, so `label` is still reported.
@Composable
fun PartlyKeyed(id: Int, label: String) {
    key(id) {
        val v = <!ComposeRememberWithoutKey!>remember<!> { id + label.length }
        use(v)
    }
}
