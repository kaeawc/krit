// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: calls to kotlin.check that are not a single `!=` null
// comparison, and null checks through other functions. Go reports none of them.
package test

fun alreadyCheckNotNull(x: Any?) {
    checkNotNull(x)
    checkNotNull(x) { "x must not be null" }
}

fun otherConditions(x: Int, y: Any?, z: Any?) {
    check(x > 0)
    check(y != z)
    check(y == null)
    check(!(y == null))
    check(y != null && z != null)
    check(y is String)
}

// Identity comparison: Go matches only the `!=` operator.
fun identity(x: Any?) {
    check(x !== null)
}

// The lazy message passed inside the parentheses makes two value arguments.
fun messageInParentheses(x: Any?) {
    check(x != null, { "x must not be null" })
    check(value = x != null, lazyMessage = { "x must not be null" })
}

// A non-null declared type: Go resolves the name and skips it.
fun nonNullParameter(x: String) {
    check(x != null)
}

fun requireIsAnotherRule(x: Any?) {
    require(x != null)
}

fun conditionFromVariable(x: Any?) {
    val present = x != null
    check(present)
}

fun notCheck(x: Any?) {
    assert(x != null)
}
