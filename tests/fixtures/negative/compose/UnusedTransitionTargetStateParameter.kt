package fixtures.compose

import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.updateTransition
import androidx.compose.runtime.Composable

@Composable
fun Fade(visible: Boolean) {
    val transition = updateTransition(visible, label = "fade")
    val alpha = transition.animateFloat(transitionSpec = { spring() }, label = "alpha") { if (it) 1f else 0f }
    val scale = transition.animateFloat(label = "scale") { state -> if (state) 1f else 0.5f }
}

class Animator {
    fun animateFloat(label: String, block: (Boolean) -> Float): Float = block(true)
}

fun notATransition(animator: Animator) {
    animator.animateFloat("x") { 1f }
}
