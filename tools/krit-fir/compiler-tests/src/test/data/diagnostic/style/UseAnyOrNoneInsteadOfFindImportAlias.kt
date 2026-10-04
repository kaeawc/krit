// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: an import alias of the stdlib function. Go reads the callee name as
// written (`locate`) and misses it; the call is still kotlin.collections.find,
// and the message names the function it resolves to.
package test

import kotlin.collections.find as locate

fun importAlias(list: List<Int>): Boolean = <!UseAnyOrNoneInsteadOfFind!>list.locate { it > 0 } != null<!>
