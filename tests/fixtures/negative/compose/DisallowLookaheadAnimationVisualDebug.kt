package fixtures.compose

import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun LookaheadAnimationVisualDebugging(content: @Composable () -> Unit) = content()

@Composable
fun Screen() {
    LookaheadAnimationVisualDebugging {
        Text("a same-file function of that name is not the debugging API")
    }
}
