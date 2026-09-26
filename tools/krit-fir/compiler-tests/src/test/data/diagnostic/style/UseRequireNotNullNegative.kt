// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: calls that are not require(x != null) on a nullable operand.
package test

fun already(x: Any?) {
    requireNotNull(x)
    requireNotNull(x) { "x must not be null" }
}

fun condition(x: Int) {
    require(x > 0)
}

fun equality(x: Any?, y: Any?) {
    require(x != y)
}

fun complex(x: String?, y: String?) {
    require(x != null && x.isNotBlank())
    require(x != null || y != null) { "one value is required" }
    require(null != x && y != null)
}

fun identity(x: Any?) {
    require(x !== null)
}

fun equalsNull(x: Any?) {
    require(x == null)
}

fun negated(x: Any?) {
    require(!(x == null))
}

fun checkIsAnotherRule(x: Any?) {
    check(x != null)
}

// Go counts the value arguments in the parentheses: a message lambda passed
// there is a second one.
fun lambdaInParentheses(x: Any?) {
    require(x != null, { "x must not be null" })
}

// The operand can never be null.
fun nonNullParam(x: String) {
    require(x != null)
}

fun inferredNonNull() {
    val s = "abc"
    require(s != null)
}

fun <T : Any> nonNullBound(x: T) {
    require(x != null)
}
