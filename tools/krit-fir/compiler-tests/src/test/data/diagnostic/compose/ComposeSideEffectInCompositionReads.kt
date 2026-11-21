// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 29, 32, 39, 44, 51, 58, 63, 68, 75, 96, 103, 110, 117, 160, 174, 181, 188, 220, 227, 234, 251
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.foundation.clickable
import androidx.compose.ui.Modifier
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

val LocalCounter = compositionLocalOf { mutableStateOf(0) }
class VM(val state: MutableState<Int>)
class Holder(var lastX: Int = 0)
fun RenderContent(content: @Composable () -> Unit) {}

// Both engines report qualified external MutableState writes; no simple receiver exemption applies.
@Composable fun CompositionLocalWrite() {
    <!ComposeSideEffectInComposition!>LocalCounter.current.value = 1<!>
}
@Composable fun QualifiedState(vm: VM) {
    <!ComposeSideEffectInComposition!>vm.state.value = 1<!>
}

// Both engines count effect key arguments as composition reads, not deferred callback reads.
@Composable fun LaunchedKey() {
    val state = remember { mutableStateOf(0) }
    LaunchedEffect(state.value) {}
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}
@Composable fun DisposableKey() {
    val state = remember { mutableStateOf(0) }
    DisposableEffect(state.value) { onDispose {} }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Both engines count remember calculations as composition reads.
@Composable fun CalculationRead() {
    val state = remember { mutableStateOf(0) }
    val derived = remember { state.value + 1 }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Both engines count a non-inline composable parameter's lambda as a composition read.
@Composable fun NonInlineContentRead() {
    val state = remember { mutableStateOf(0) }
    RenderContent { val old = state.value }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Both engines report the external nullable holder write; last* only exempts an implicit receiver capture.
@Composable fun SafeCapture(holder: Holder?, input: Int) {
    <!ComposeSideEffectInComposition!>holder?.lastX = input<!>
}

// Both engines report the implicit MutableState extension receiver write.
@Composable fun MutableState<Int>.ImplicitWrite() {
    <!ComposeSideEffectInComposition!>value = 1<!>
}

// Both engines report remembered saveable state after a calculation/key read; the all-rules smoke keeps only a callback write.
@Composable fun SaveableRead() {
    var text by rememberSaveable { mutableStateOf("") }
    val length = rememberSaveable(text) { text.length }
    <!ComposeSideEffectInComposition!>text = "x$length"<!>
}

// Go never inspects lambdas; FIR reports because this read clearly precedes the write in composition order.
@Composable fun ReadBeforeContentWrite() {
    val state = remember { mutableStateOf(0) }
    val old = state.value
    Column { <!ComposeSideEffectInComposition!>state.value = 1<!> }
}

// A later composition read does not justify a write-after-read report in either engine.
@Composable fun ReadAfterContentWrite() {
    val state = remember { mutableStateOf(0) }
    Column { state.value = 1 }
    val old = state.value
}

// Both engines conservatively count earlier reads in property-initializer lambdas, even when not invoked.
@Composable fun CallbackRead() {
    val state = remember { mutableStateOf(0) }
    val callback = { println(state.value) }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Both engines count earlier derivedStateOf reads inside remember calculations.
@Composable fun DerivedStateRead() {
    val state = remember { mutableStateOf(0) }
    val derived = remember { derivedStateOf { state.value } }
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Both engines conservatively count earlier reads inside non-invoked local functions.
@Composable fun LocalFunctionRead() {
    val state = remember { mutableStateOf(0) }
    fun peek() = state.value
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

// Local composable functions also contribute earlier reads, without a separate body walk per lambda.
@Composable fun LocalComposableRead() {
    val state = remember { mutableStateOf(0) }
    @Composable fun Peek() = state.value
    <!ComposeSideEffectInComposition!>state.value = 1<!>
}

fun Button(onClick: () -> Unit) {}
fun Modifier.onSizeChanged(onSizeChanged: () -> Unit): Modifier = this
fun Modifier.pointerInput(key: Any?, block: suspend () -> Unit): Modifier = this
operator fun (() -> Unit).plus(next: () -> Unit): () -> Unit = { this(); next() }

// Both engines ignore reads inside named onClick arguments; the remembered write stays silent.
@Composable fun ButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = { println(holder.value) })
    holder.value = input
}

// Both engines ignore reads inside clickable, launch and pointerInput lambdas, not their composition arguments.
@Composable fun ClickableRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.clickable { holder.value }
    holder.value = input
}
@Composable fun LaunchRead(scope: CoroutineScope, input: Int) {
    val holder = remember { mutableStateOf(0) }
    scope.launch { holder.value }
    holder.value = input
}
@Composable fun PointerInputRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.pointerInput(Unit) { holder.value }
    holder.value = input
}

// Both engines defer lambda literals nested in deferred calls; the conditional branches remain silent.
@Composable fun IfPointerInputRead(flag: Boolean, input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.pointerInput(Unit, if (flag) { { holder.value } } else { {} })
    holder.value = input
}

// Both engines count anonymous functions passed to deferred calls; only lambda literals are deferred.
@Composable fun AnonymousClickableCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.clickable(onClick = fun() { holder.value })
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// Both engines defer lambdas through operator expressions nested in named onX arguments.
@Composable fun PlusButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = {} + { println(holder.value) })
    holder.value = input
}

// Both engines count trailing on* lambdas without a written label as composition reads.
@Composable fun TrailingOnSizeChangedRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.onSizeChanged { println(holder.value) }
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// Both engines count positional callback lambdas; only written onX labels are deferred by Go.
@Composable fun PositionalButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button({ holder.value })
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// Both engines count anonymous functions because they are not lambda literals.
@Composable fun AnonymousButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = fun() { holder.value })
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// Both engines ignore reads in parenthesized and labeled named callbacks.
@Composable fun WrappedButtonCallbackReads(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = ({ println(holder.value) }))
    Button(onClick = cb@{ println(holder.value) })
    holder.value = input
}

// Both engines defer lambda literals through non-call wrappers in named onX arguments, including Elvis, if, and cast expressions.
@Composable fun ElvisButtonCallbackRead(override: (() -> Unit)?, input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = override ?: { println(holder.value) })
    holder.value = input
}
@Composable fun IfButtonCallbackRead(flag: Boolean, input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = if (flag) { { println(holder.value) } } else { {} })
    holder.value = input
}
@Composable fun CastButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = { println(holder.value) } as () -> Unit)
    holder.value = input
}

// Both engines count the let lambda because its call runs during composition.
@Composable fun QualifiedLetButtonRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = Unit.let { val seen = holder.value; { println(seen) } })
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// FIR conservatively defers the safe-call receiver lambda.
@Composable fun SafeLetReceiverButtonRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = { holder.value }?.let { r -> val seen = r(); { println(seen) } } ?: {})
    holder.value = input
}

// FIR defers escaped callback reads; Go still reports this write (pinned in go-lines).
@Composable fun EscapedButtonCallbackRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(`onClick` = { println(holder.value) })
    holder.value = input
}

// Deferred reads also leave writes inside composable lambdas silent in both engines; Go never inspects lambdas.
@Composable fun DeferredReadsBeforeContent(scope: CoroutineScope, input: Int) {
    val holder = remember { mutableStateOf(0) }
    Button(onClick = { println(holder.value) })
    Modifier.clickable { holder.value }
    scope.launch { holder.value }
    Modifier.pointerInput(Unit) { holder.value }
    Column { holder.value = input }
}

// Both engines still count a pointerInput key read during composition, outside its deferred lambda.
@Composable fun PointerInputKeyRead(input: Int) {
    val holder = remember { mutableStateOf(0) }
    Modifier.pointerInput(holder.value) { holder.value }
    <!ComposeSideEffectInComposition!>holder.value = input<!>
}

// Deferred callbacks still contribute local declarations; both engines stay silent for these plain-local writes.
@Composable fun DeferredLocalDeclarations() {
    Button(onClick = {
        val content: @Composable () -> Unit = {
            val cache = mutableMapOf<String, Int>()
            cache["key"] = 1
        }
    })
}
