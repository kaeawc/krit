// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 12
// Calls to the stdlib factories that pass an element Go does not count.
package test

// Divergence: the trailing lambda is the element of the single-element
// overload `listOf(element)`, so this is a one-element list of a function,
// not an empty list. Go counts only the arguments inside the parentheses and
// reports it.
fun trailingLambda() {
    val a = listOf() { 1 }
    val b = setOf() { 2 }
    // Neither reports the lambda-only call: Go sees no argument list.
    val c = setOf { 3 }
    println(listOf(a, b, c))
}
