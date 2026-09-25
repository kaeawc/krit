// Smoke: animate*AsState delegate idiom and MutableTransitionState.
package stubs

import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember

@Composable
fun FadingThing(visible: Boolean) {
    val alpha by animateFloatAsState(
        targetValue = if (visible) 1f else 0f,
        animationSpec = tween(durationMillis = 300),
        label = "alpha",
    )
    val bounce by animateFloatAsState(if (visible) 2f else 1f, spring(dampingRatio = 0.5f))
    val state = remember { MutableTransitionState(false) }
    state.targetState = true
    println("$alpha $bounce ${state.currentState} ${state.isIdle}")
}
