package fixtures.positive.potentialbugs

class UselessElvisOnNonNull {
    fun example() {
        val x: String = "hello"
        val y = x ?: "fallback"
    }
}
