// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: throws check() and error() cannot replace, and other exception types.
package test.usecheckorerrornegative

fun withCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalStateException("bad", cause)
}

fun onlyCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalStateException(cause)
}

fun nullableMessage(cond: Boolean, message: String?) {
    if (cond) throw IllegalStateException(message)
}

fun exceptionMessage(e: Exception) {
    throw IllegalStateException(e.message)
}

fun wrongType(cond: Boolean) {
    if (!cond) throw RuntimeException()
}

fun subclass(cond: Boolean) {
    if (!cond) throw UnsupportedOperationException("nope")
}

fun illegalArgument(cond: Boolean) {
    if (!cond) throw IllegalArgumentException("not this rule")
}

fun rethrow(e: IllegalStateException) {
    throw e
}

fun created(): Exception = IllegalStateException("not thrown")

fun onlyStatementInLambda(items: List<Int>) {
    items.forEach { throw IllegalStateException("only statement") }
}

fun onlyStatementInLambdaWithParameter(items: List<Int>) {
    items.forEach { item -> throw IllegalStateException("only statement $item") }
}

// A `;` is not a statement, so the throw is still the lambda's only statement.
fun onlyStatementInLambdaWithSemicolon(items: List<Int>) {
    items.forEach { throw IllegalStateException("trailing"); }
    items.forEach { ; throw IllegalStateException("leading") }
}

fun builtOutsideThrow(cond: Boolean) {
    val error = IllegalStateException("built first")
    if (cond) throw error
}

fun helperThrows(cond: Boolean) {
    if (cond) throw fail(IllegalStateException("wrapped"))
}

fun fail(e: Exception): Exception = e

fun usesCheck(valid: Boolean) {
    check(valid) { "bad" }
    error("also fine")
}
