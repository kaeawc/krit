// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Recall: import aliases of kotlin.collections.filter and first still call
// the stdlib functions; Go compares the call names and misses them. The
// message names the stdlib functions.
package test

import kotlin.collections.filter as keep
import kotlin.collections.first as head

fun aliased(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.keep { it > 0 }.head()

fun aliasedMultiLine(list: List<Int>): Int =
    <!UnnecessaryFilter!>list<!>
        .keep { it > 1 }
        .head()
