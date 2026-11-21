// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 24, 25, 26, 33, 40, 47, 87, 94, 101, 103, 105, 106, 107, 108, 131, 137, 145, 165, 166, 186
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Composable as RuntimeComposable
import androidx.compose.runtime.MutableFloatState
import androidx.compose.runtime.MutableIntState
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf

class Holder(var seen: Boolean = false, var value: Int = 0)
val LocalHolder = staticCompositionLocalOf { Holder() }

@Composable
fun Direct(holder: Holder, cache: MutableMap<String, Int>) {
    <!ComposeSideEffectInComposition!>holder.seen = true<!>
    <!ComposeSideEffectInComposition!>holder.value = 1<!>
    <!ComposeSideEffectInComposition!>LocalHolder.current.seen = true<!>
    <!ComposeSideEffectInComposition!>cache["key"] = 1<!>
}

@Composable
fun RememberedRead() {
    val state = remember { mutableStateOf(0) }
    val old = state.value
    <!ComposeSideEffectInComposition!>state.value = old + 1<!>
}

@Composable
fun DelegatedRead() {
    var count by remember { mutableStateOf(0) }
    val old = count
    <!ComposeSideEffectInComposition!>count = old + 1<!>
}

class Presenter {
    var selected = false
    @Composable
    fun Render(selected: Boolean) {
        <!ComposeSideEffectInComposition!>this.selected = selected<!>
    }
}

// Go exempts every remember*/animate* RHS name, even these project functions.
fun rememberCounter(): Int = 1
fun animateCounter(): Int = 1

@Composable
fun InitializerLookalikes(holder: Holder) {
    <!ComposeSideEffectInComposition!>holder.value = rememberCounter()<!>
    <!ComposeSideEffectInComposition!>holder.value = animateCounter()<!>
}

// Go requires @Composable on a named enclosing function; FIR also sees lambda type annotations.
fun RenderContent(content: @Composable () -> Unit) {}
fun ComposableLambdas(holder: Holder) {
    val content: @Composable () -> Unit = {
        <!ComposeSideEffectInComposition!>holder.seen = true<!>
    }
    RenderContent {
        <!ComposeSideEffectInComposition!>holder.seen = true<!>
    }
    val explicit = @Composable {
        <!ComposeSideEffectInComposition!>holder.seen = true<!>
    }
}

// Go's file-wide MutableState name evidence confuses a different declaration with this receiver.
val shadowed: MutableState<Int> = mutableStateOf(0)
@Composable
fun ShadowedReceiver(shadowed: Holder) {
    <!ComposeSideEffectInComposition!>shadowed.value = 1<!>
}

// Go counts a read in the synchronous run block, and so does FIR (it is not a callback).
@Composable
fun InlineRead() {
    val state = remember { mutableStateOf(0) }
    run { val old = state.value }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

@Composable
fun RememberedMapRead() {
    val cache = remember { mutableMapOf<String, Int>() }
    val old = cache["key"]
    <!ComposeSideEffectInComposition!>cache["key"] = 1<!>
}

// Fresh local primitive arrays are safe, but Go still reports those writes (pinned in go-lines); caller-owned arrays remain reportable.
@Composable
fun PrimitiveArrayWrites(bytesFromCaller: ByteArray, charsFromCaller: CharArray, shortsFromCaller: ShortArray) {
    val bytes = ByteArray(4)
    bytes[0] = 1
    val chars = CharArray(4)
    chars[0] = 'a'
    val shorts = ShortArray(4)
    shorts[0] = 1
    <!ComposeSideEffectInComposition!>bytesFromCaller[0] = 1<!>
    <!ComposeSideEffectInComposition!>charsFromCaller[0] = 'a'<!>
    <!ComposeSideEffectInComposition!>shortsFromCaller[0] = 1<!>
}

class IntValueHolder(var intValue: Int = 0)

@Composable
fun RememberedPrimitiveStateWrite() {
    val intState = remember { mutableIntStateOf(0) }
    val oldInt = intState.intValue
    <!ComposeSideEffectInComposition!>intState.intValue = oldInt + 1<!>

    val floatState = remember { mutableFloatStateOf(0f) }
    val oldFloat = floatState.floatValue
    <!ComposeSideEffectInComposition!>floatState.floatValue = oldFloat + 1f<!>

    val holder = remember { IntValueHolder() }
    val oldValue = holder.intValue
    holder.intValue = oldValue + 1
}

// FIR suppresses these primitive-state writes; Go reports them as assignments.
@Composable
fun ParamWrite(state: MutableIntState) {
    state.intValue = 1
}

@Composable
fun FreshWrite() {
    val s = mutableIntStateOf(0)
    s.intValue = 1
}

class P {
    val s = mutableFloatStateOf(0f)

    @Composable
    fun R() {
        s.floatValue = 1f
    }
}

// Go does not recognize the annotation alias, but FIR resolves the real Compose annotation.
@RuntimeComposable
fun AliasedAnnotation(holder: Holder) {
    <!ComposeSideEffectInComposition!>holder.seen = true<!>
}

typealias Content = @Composable () -> Unit
// Go does not see composable types through aliases; FIR sees the expanded annotation.
fun AliasedLambda(holder: Holder) {
    val content: Content = {
        <!ComposeSideEffectInComposition!>holder.seen = true<!>
    }
}

@Composable
fun CompoundWrites(state: MutableState<Int>, items: MutableList<Int>) {
    <!ComposeSideEffectInComposition!>state.value += 1<!>
    <!ComposeSideEffectInComposition!>items += 1<!>
}

// Go's remembered-object synchronization heuristic exempts even this state increment; it is a recomposition side effect.
@Composable
fun RememberedIncrement() {
    val state = remember { mutableStateOf(0) }
    <!ComposeSideEffectInComposition!>state.value += 1<!>
}

// FIR recognizes primitive compound writes as reads; Go exempts remembered-object increments.
@Composable
fun RememberedIntIncrement() {
    val state = remember { mutableIntStateOf(0) }
    <!ComposeSideEffectInComposition!>state.intValue += 1<!>
}

@Composable
fun AnonymousHolder() {
    val holder = object { var value = 0 }
    <!ComposeSideEffectInComposition!>holder.value = 1<!>
}
