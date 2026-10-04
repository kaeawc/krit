// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A parenthesized `(null)` is still the null literal, so each marked call
// compares a nullable operand with null and should be requireNotNull(x), as
// UseCheckNotNull reports `check(x != (null))`. Go misses them because it
// compares the operand's raw text with "null".
package test

fun parenthesizedNull(x: Any?) {
    <!UseRequireNotNull!>require(x != (null))<!>
}

fun parenthesizedNullReversed(x: String?) {
    <!UseRequireNotNull!>require((null) != x)<!>
}

fun doublyParenthesizedNull(x: String?) {
    <!UseRequireNotNull!>require(x != ((null)))<!>
}

// Not reported, in Go or here: the operand cannot be null.
fun nonNullOperand(x: String) {
    require(x != (null))
}
