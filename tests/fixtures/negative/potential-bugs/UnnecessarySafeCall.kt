package fixtures.negative.potentialbugs

class UnnecessarySafeCall {
    fun example(nullable: String?) {
        val len = nullable?.length
    }

    // Chained safe calls — upstream ?. makes receiver nullable
    fun chainedSafeCalls(content: Content?) {
        val seconds = content?.dataMessage?.expireTimer?.seconds
    }

    // Dotted member access — bare name resolution is unreliable
    fun dottedAccess(obj: Wrapper) {
        val value = obj.property?.name
    }

    // Nullable parameter with default — safe call is justified
    fun withNullableDefault(s: String? = null) {
        val len = s?.length
    }

    // String literals containing "this?." inside a lambda body must NOT
    // trip the repeated-`this?.` heuristic that suppresses findings on
    // `this?.X` in scope-function lambdas. The rule already does not flag
    // `this?.X` directly, but downstream callers consult the helper to
    // decide whether the receiver-this is nullable; an AST-based count
    // ensures only real navigation_expressions contribute.
    fun lambdaWithStringLiteralLookingLikeThisSafeCalls(obj: String?) {
        obj?.let {
            // Two textual occurrences of "this?." inside a single literal —
            // must not count as repeated AST navigations.
            val msg = "doc: this?.foo and this?.bar are not real calls"
            println(msg)
        }
    }
}
