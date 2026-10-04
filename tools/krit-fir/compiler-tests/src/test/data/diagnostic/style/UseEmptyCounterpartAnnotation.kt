// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 14, 18, 22, 26
// `arrayOf()` in annotation arguments and annotation parameter defaults. K2
// turns these calls into collection literals; `emptyArray()` is accepted in
// the same places, so each empty `arrayOf()` is reported like Go does.
package test

annotation class Names(val values: Array<String> = <!UseEmptyCounterpart!>arrayOf<!>())

annotation class Outer(val inner: Names)

annotation class Many(vararg val names: String)

@Names(<!UseEmptyCounterpart!>arrayOf<!>())
fun positional() {
}

@Names(values = <!UseEmptyCounterpart!>arrayOf<!><String>())
fun named() {
}

@Outer(Names(<!UseEmptyCounterpart!>arrayOf<!>()))
fun nested() {
}

@Many(*<!UseEmptyCounterpart!>arrayOf<!>())
fun spread() {
}

// Go misses the qualified call: the callee is a navigation expression.
@Names(kotlin.<!UseEmptyCounterpart!>arrayOf<!>())
fun qualified() {
}

// Negative: an element, the counterpart itself, a literal, another factory.
@Names(arrayOf("a"))
fun withElement() {
}

@Names(emptyArray())
fun counterpart() {
}

@Names([])
fun literal() {
}

annotation class Ints(val values: IntArray)

@Ints(intArrayOf())
fun ints() {
}

@Many
fun noArgument() {
}
