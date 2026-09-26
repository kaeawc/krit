// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Calls to the stdlib factories that Go misses because the callee is not a
// plain identifier spelled like the factory. Each is still an empty call to
// the factory, so the empty counterpart says the same thing.
package test

import kotlin.arrayOf as noArray
import kotlin.collections.setOf as noElements

// Go misses the qualified calls: the callee is a navigation expression.
fun qualified() {
    val a = kotlin.collections.<!UseEmptyCounterpart!>listOf<!><String>()
    val b = kotlin.<!UseEmptyCounterpart!>arrayOf<!><String>()
    val c = kotlin.sequences.<!UseEmptyCounterpart!>sequenceOf<!><Int>()
    println(listOf(a, b, c))
}

// Go misses the backticked call: its identifier text keeps the backticks.
fun backticked() {
    val a = <!UseEmptyCounterpart!>`listOf`<!><String>()
    println(a)
}

// Go misses the import alias: it matches the written name. The message names
// the factory the alias stands for.
fun aliased() {
    val a = <!UseEmptyCounterpart!>noElements<!><Int>()
    println(a)
}

annotation class Tags(val values: Array<String>)

// Go misses the import alias in an annotation argument too.
@Tags(<!UseEmptyCounterpart!>noArray<!>())
fun aliasedInAnnotation() {
    val a: Array<String> = <!UseEmptyCounterpart!>noArray<!>()
    println(a)
}
