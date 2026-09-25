// Compiler-test source stubs; never packaged in the production artifact.
package androidx.compose.animation.core

import androidx.compose.runtime.Composable
import androidx.compose.runtime.State

sealed class TransitionState<S> {
    abstract val currentState: S

    abstract val targetState: S
}

class MutableTransitionState<S>(initialState: S) : TransitionState<S>() {
    override val currentState: S
        get() = TODO()

    override var targetState: S
        get() = TODO()
        set(value) = TODO()

    val isIdle: Boolean
        get() = TODO()
}

interface AnimationSpec<T>

interface FiniteAnimationSpec<T> : AnimationSpec<T>

class TweenSpec<T>(val durationMillis: Int = 300, val delay: Int = 0) : FiniteAnimationSpec<T>

class SpringSpec<T>(val dampingRatio: Float = 1f, val stiffness: Float = 1500f, val visibilityThreshold: T? = null) :
    FiniteAnimationSpec<T>

fun <T> tween(durationMillis: Int = 300, delayMillis: Int = 0): TweenSpec<T> = TODO()

fun <T> spring(dampingRatio: Float = 1f, stiffness: Float = 1500f, visibilityThreshold: T? = null): SpringSpec<T> =
    TODO()

@Composable
fun animateFloatAsState(
    targetValue: Float,
    animationSpec: AnimationSpec<Float> = spring(),
    visibilityThreshold: Float = 0.01f,
    label: String = "FloatAnimation",
    finishedListener: ((Float) -> Unit)? = null,
): State<Float> = TODO()
