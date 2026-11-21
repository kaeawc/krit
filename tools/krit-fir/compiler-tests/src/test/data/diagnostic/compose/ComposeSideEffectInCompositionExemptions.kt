// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 45, 61, 62, 105, 111, 126, 133
package test

import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.runtime.remember
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.tooling.preview.Preview

class Holder(var seen: Boolean = false, var value: Int = 0, var v: Int = 0)

// Go matches the qualified annotation's short name; FIR resolves it to a project lookalike.
object Annotations { annotation class Composable }
@Annotations.Composable
fun AnnotationLookalike(holder: Holder) {
    holder.seen = true
}

// Both engines exempt callbacks, initializers, nested functions and conditional writes.
@Composable
fun Boundaries(holder: Holder, state: MutableState<Int>) {
    LaunchedEffect(Unit) { holder.seen = true }
    SideEffect { holder.seen = true }
    val initialized = remember { Holder().apply { seen = true } }
    val callback = { holder.seen = true }
    fun local() { holder.seen = true }
    if (state.value > 0) { holder.seen = true }
}

@Composable
fun FreshAndSynchronization(input: Int) {
    val state = remember { mutableStateOf(0) }
    state.value = input
    val holder = remember { Holder() }
    val old = holder.value
    // Go reports any remembered .value after a read; FIR stays silent for ordinary holder synchronization.
    holder.value = input
    val saved = rememberSaveable { mutableStateOf(0) }
    saved.value = input
    var count by remember { mutableStateOf(0) }
    count = input
    val cache = remember { mutableMapOf<String, Int>() }
    cache["key"] = input
}

typealias StateAlias = MutableState<Int>
interface SpecializedState : MutableState<Int>

// Go reports subtype/safe-call writes; file-wide state: MutableState evidence exempts the alias. FIR resolves and exempts all three.
@Composable
fun ResolvedState(state: StateAlias, subtype: SpecializedState, nullable: StateAlias?) {
    state.value = 1
    subtype.value = 1
    nullable?.value = 1
}

@Composable
fun LocalAndTransition(transition: MutableTransitionState<Boolean>) {
    var count = 0
    count = 1
    val cache = mutableMapOf<String, Int>()
    cache["key"] = count
    transition.targetState = true
}

class Presenter {
    var lastInput = 0
    @Composable
    fun Capture(input: Int) {
        lastInput = input
    }
}

@Composable
fun Documented(holder: Holder) {
    // Safe to repeat: explicitly eager synchronization.
    holder.value = 1
}

@Preview
@Composable
fun PreviewSetup(holder: Holder) {
    holder.seen = true
}

class FakePresenter {
    @Composable
    fun Render(holder: Holder) {
        holder.seen = true
    }
}

// Go's holder-sync check misses rememberSaveable and reports this write (its fresh-state check matches the factory); FIR stays silent.
@Composable
fun SavedHolder(input: Int) {
    val holder = rememberSaveable { Holder() }
    holder.seen = input > 0
}

// Go cannot extract a trailing-lambda RHS call; FIR recognizes the Compose initializer and stays silent.
@Composable
fun RememberRhs(holder: Holder) {
    holder.v = remember { 0 }
}

// Both engines exempt a Compose animation initializer, including its .value access.
class AnimationHolder(var value: Float)
@Composable
fun AnimationRhs(holder: AnimationHolder) {
    holder.value = animateFloatAsState(1f).value
}

fun rememberFoo(): Int = 0
// Go treats any remember-prefixed factory as escaping state; FIR stays silent for plain local rebinding.
@Composable
fun LocalRememberLookalike() {
    var count = rememberFoo()
    count = 1
}

class VM(val transition: MutableTransitionState<Boolean>)
// Go cannot identify the qualified transition receiver; FIR proves the transition type and stays silent.
@Composable
fun QualifiedTransition(vm: VM) {
    vm.transition.targetState = true
}
