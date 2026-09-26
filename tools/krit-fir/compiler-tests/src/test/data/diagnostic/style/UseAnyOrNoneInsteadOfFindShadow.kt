// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12
// A same-package extension that shadows the stdlib find. The receiver is still
// a List, which has the stdlib any / none, so the message is true: Go reports
// both comparisons and the checker keeps them.
package test

fun <T> List<T>.find(predicate: (T) -> Boolean): T? = firstOrNull(predicate)

fun shadowNotNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } != null<!>

fun shadowIsNull(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.find { it > 0 } == null<!>
