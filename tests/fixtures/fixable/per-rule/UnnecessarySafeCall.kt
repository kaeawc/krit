package fixtures.positive.potentialbugs

class UnnecessarySafeCall {
    fun example() {
        val x: String = "hello"
        val len = x?.length
    }

    fun exampleWithMisleadingComment() {
        val y: String = "world"
        // Regression: a comment containing "?." between the receiver and
        // the real safe-call operator must not confuse the autofix.
        val len = y /* ?.fake */ ?.length
    }
}
