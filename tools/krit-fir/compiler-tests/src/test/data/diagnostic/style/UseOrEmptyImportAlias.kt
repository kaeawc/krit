// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An import alias of kotlin.collections.emptyList still names the empty
// list. Go compares the callee name and misses it.
package test

import kotlin.collections.emptyList as noItems

fun aliased(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: noItems()<!>
