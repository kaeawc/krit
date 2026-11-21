// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 22, 29, 37, 75, 77, 84
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.SideEffect as Deferred
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember as Memo
import androidx.compose.runtime.remember
import androidx.compose.animation.core.MutableTransitionState as Transition

class Holder(var value: Int = 0)
typealias StateAlias = MutableState<Int>
interface SpecializedState : MutableState<Int>

// Go has no MutableState evidence for aliased/subtyped/nullable receivers; FIR resolves their types.
@Composable
fun ResolvedState(aliased: StateAlias, subtype: SpecializedState, nullable: StateAlias?) {
    aliased.value = 1
    subtype.value = 1
    nullable?.value = 1
}

// Go does not recognize an aliased remember, so it reports remembered-object synchronization.
@Composable
fun AliasedRemember(input: Int) {
    val holder = Memo { Holder() }
    holder.value = input
}

// Go reads the state inside the aliased SideEffect as an earlier composition read. FIR resolves the effect.
@Composable
fun AliasedEffectRead() {
    val state = remember { mutableStateOf(0) }
    Deferred { val old = state.value }
    state.value = 1
}

// Go mistakes this project remember function for Compose remember and exempts the external property write.
@Composable
fun RememberLookalike() {
    fun <T> remember(block: () -> T): T = block()
    val holder = remember { Holder() }
    <!ComposeSideEffectInComposition!>holder.value = 1<!>
}

// Both engines suppress reads in any SideEffect-named lambda, even this synchronous project lookalike.
inline fun SideEffect(block: () -> Unit) { block() }
@Composable
fun EffectLookalikeRead() {
    val state = remember { mutableStateOf(0) }
    SideEffect { val old = state.value }
    state.value = 1
}

// Go exempts a same-named project type because the real transition type is imported (under an alias).
class MutableTransitionState<T>(var targetState: T)
@Composable
fun TransitionLookalike(state: MutableTransitionState<Boolean>) {
    <!ComposeSideEffectInComposition!>state.targetState = true<!>
}

class Nested(val holder: Holder)
@Composable
fun NestedRememberSynchronization(input: Int) {
    val outer = remember { Nested(Holder()) }
    outer.holder.value = input
}

// Go cannot extract a plain local's name from a compound LHS. These resolved local writes do not escape composition.
@Composable
fun PlainLocalCompoundWrites() {
    var count = 0
    count += 1
    val items = mutableListOf<Int>()
    items += count
}

// Go excludes mutableState* initializers from its plain-local exemption; rebinding this local is not a state mutation.
@Composable
fun LocalStateRebinding() {
    var state = mutableStateOf(0)
    state = mutableStateOf(1)
}

typealias ComposeAnnotation = Composable
// Go cannot resolve an annotation type alias on a lambda type; FIR expands the annotation class ID.
fun AnnotationAliasLambda(holder: Holder) {
    val content: @ComposeAnnotation () -> Unit = {
        <!ComposeSideEffectInComposition!>holder.value = 1<!>
    }
}
