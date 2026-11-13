// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27
// Recall: the callee and its type are resolved, so the spelling at the call site does not matter.
package test

import androidx.annotation.CheckResult as Checked
import kotlin.sequences.filter as lazyFilter

class Store {
    @Checked
    fun snapshot(): String = "x"
}

typealias Numbers = Sequence<Int>

fun numbers(): Numbers = sequenceOf(1)

fun importAlias(items: Sequence<Int>) {
    items.<!IgnoredReturnValue!>lazyFilter<!> { it > 1 }
}

fun annotationAlias(store: Store) {
    store.<!IgnoredReturnValue!>snapshot<!>()
}

fun typeAlias() {
    numbers().<!IgnoredReturnValue!>map<!> { it + 1 }
}

fun qualified(items: Sequence<Int>) {
    kotlin.sequences.<!IgnoredReturnValue!>emptySequence<!><Int>()
    items.<!IgnoredReturnValue!>onEach<!> { println(it) }
}

fun javaStream(items: List<Int>) {
    items.stream().<!IgnoredReturnValue!>map<!> { it + 1 }
}
