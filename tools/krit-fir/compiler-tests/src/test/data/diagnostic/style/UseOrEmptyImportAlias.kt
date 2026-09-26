// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An import alias of kotlin.collections.emptyList still names the empty
// list. Go compares the callee name and misses it.
package test

import kotlin.collections.emptyList as noItems
import kotlin.emptyArray as noValues

fun aliased(x: List<String>?): List<String> = <!UseOrEmpty!>x ?: noItems()<!>

// An alias of kotlin.emptyArray is not reported: Go does not see it, and
// `.orEmpty()` returns `Array<out Int>`, which the invariant `Array<Int>`
// return type rejects.
fun aliasedArray(x: Array<Int>?): Array<Int> = x ?: noValues()
