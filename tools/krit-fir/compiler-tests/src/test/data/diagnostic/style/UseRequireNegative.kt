// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: throws that are not the whole body of a conditional, throws with a cause, and other exception types.
package test.userequirenegative

fun withCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalArgumentException("bad", cause)
}

fun onlyCause(cond: Boolean, cause: Exception) {
    if (cond) throw IllegalArgumentException(cause)
}

fun nullableMessage(cond: Boolean, message: String?) {
    if (cond) throw IllegalArgumentException(message)
}

fun multipleStatements(cond: Boolean) {
    if (!cond) {
        println("log")
        throw IllegalArgumentException()
    }
}

fun unconditional() {
    throw IllegalArgumentException("always")
}

fun elvis(x: String?): String = x ?: throw IllegalArgumentException("missing")

fun whenBranch(x: Int): String = when (x) {
    0 -> "zero"
    else -> throw IllegalArgumentException("unexpected $x")
}

fun whenBranchBraced(x: Int): String = when (x) {
    0 -> "zero"
    else -> {
        throw IllegalArgumentException("unexpected $x")
    }
}

fun expressionBody(): Nothing = throw IllegalArgumentException("never")

fun lambdaBody(items: List<Int>) {
    items.forEach { throw IllegalArgumentException("lambda") }
}

fun wrongType(cond: Boolean) {
    if (!cond) throw RuntimeException()
}

fun illegalState(cond: Boolean) {
    if (!cond) throw IllegalStateException("not this rule")
}

fun rethrow(cond: Boolean, e: IllegalArgumentException) {
    if (cond) throw e
}

fun usesRequire(valid: Boolean) {
    require(valid) { "bad" }
}
