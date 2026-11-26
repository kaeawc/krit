package fixtures.compose

import androidx.compose.animation.ExperimentalLookaheadAnimationVisualDebugApi
import androidx.compose.animation.LookaheadAnimationVisualDebugging
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@OptIn(ExperimentalLookaheadAnimationVisualDebugApi::class)
@Composable
fun Screen() {
    LookaheadAnimationVisualDebugging {
        Text("debugging left on")
    }
}
