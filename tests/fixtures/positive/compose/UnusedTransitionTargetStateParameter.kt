package fixtures.compose

import androidx.compose.animation.core.Transition
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.updateTransition
import androidx.compose.runtime.Composable

@Composable
fun Fade(visible: Boolean, enabled: Boolean) {
    val transition = updateTransition(visible, label = "fade")
    val alpha = transition.animateFloat(label = "alpha") { if (enabled) 1f else 0f }
}

@Composable
fun Transition<Boolean>.scale(): Float = animateFloat(label = "scale") { state -> 1f }.value
