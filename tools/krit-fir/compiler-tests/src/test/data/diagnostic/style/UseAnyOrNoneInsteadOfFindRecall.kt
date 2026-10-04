// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: stdlib find / firstOrNull / lastOrNull compared with null in shapes
// Go misses because it needs `receiver.name { ... }` written with a trailing
// lambda and a bare `null` literal. The comparison is the same idiom, and
// `any` / `none` take the same predicate.
package test

// Go misses these: the call has no explicit receiver.
fun implicitReceiver(list: List<Int>): Boolean = with(list) { <!UseAnyOrNoneInsteadOfFind!>find { it > 0 } != null<!> }

class Numbers : ArrayList<Int>() {
    fun inherited(): Boolean = <!UseAnyOrNoneInsteadOfFind!>firstOrNull { it > 0 } == null<!>
}

// Go misses these: the lambda is inside the parentheses, not trailing.
fun parenthesizedLambda(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find({ it > 0 }) != null<!>

fun namedLambda(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.lastOrNull(predicate = { it > 0 }) == null<!>

// Go misses these: the call or the null is parenthesized.
fun parenthesizedCall(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>(list.find { it > 0 }) != null<!>

fun parenthesizedNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != (null)<!>

// Go misses this: the name is backticked.
fun backticked(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.`find` { it > 0 } != null<!>
