// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: import aliases of kotlin.collections.filter and first. K2 drops
// an alias-imported name from the default imports, so after
// `import kotlin.collections.first as head` the suggested `.first { }` does
// not resolve here, and the message would quote `.filter {..}.first()`,
// which is not in the code. Go compares the call names and skips them too.
package test

import kotlin.collections.filter as keep
import kotlin.collections.first as head

fun aliased(list: List<Int>): Int = list.keep { it > 0 }.head()

fun aliasedMultiLine(list: List<Int>): Int =
    list
        .keep { it > 1 }
        .head()
