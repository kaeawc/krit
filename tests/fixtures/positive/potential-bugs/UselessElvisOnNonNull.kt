package fixtures.positive.potentialbugs

class UselessElvisOnNonNull {
    fun nonNullLocal() {
        val x: String = "hello"
        val y = x ?: "fallback"
    }

    fun nonNullLiteral() {
        val y = "always" ?: "dead"
    }

    fun nonNullInt() {
        val n: Int = 42
        val m = n ?: 0
    }
}
