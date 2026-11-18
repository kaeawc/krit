// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 30, 37, 56, 64
// A write-back re-syncs the remembered object only when it assigns a member of that object (`it.x`, `h.x`,
// `this.x` or an implicit `x` in apply) and its value reads the parameter. A write to an outer variable, or a
// comment or string that only names the parameter, leaves the object stale, and so does a setter that runs
// only in a callback rather than on each recomposition or in an effect.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember

fun use(x: Any?) {}

class Holder(var value: Int) {
    var note: String = ""
    fun setLevel(next: Int) { value = next }
}

var lastSeen = 0

@Composable
fun Outer(input: Int) {
    val holder = <!ComposeRememberWithoutKey!>remember<!> { Holder(input) }.also { lastSeen = input }
    use(holder)
}

@Composable
fun Quoted(input: Int) {
    val holder = <!ComposeRememberWithoutKey!>remember<!> { Holder(input) }.also { it.note = "input" /* input */ }
    use(holder)
}

@Composable
fun Local(input: Int) {
    var seen = 0
    val holder = <!ComposeRememberWithoutKey!>remember<!> { Holder(input) }.apply { seen = input }
    use(holder)
    use(seen)
}

@Composable
fun Named(input: Int) {
    val holder = remember { Holder(input) }.also { h -> h.value = input }
    use(holder)
}

@Composable
fun Member(input: Int) {
    val holder = remember { Holder(input) }.apply { value = input }
    use(holder)
}

@Composable
fun Setter(input: Int) {
    val holder = <!ComposeRememberWithoutKey!>remember<!> { Holder(input) }
    holder.setLevel(0) // input
}

fun onClick(action: () -> Unit) = action()

@Composable
fun Callback(input: Int) {
    val holder = <!ComposeRememberWithoutKey!>remember<!> { Holder(input) }
    onClick { holder.setLevel(input) }
}

@Composable
fun Effect(input: Int) {
    val holder = remember { Holder(input) }
    LaunchedEffect(input) { holder.setLevel(input) }
}
