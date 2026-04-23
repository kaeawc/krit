package fixtures.positive.potentialbugs

class UnnecessarySafeCall {
    fun example() {
        val x: String = "hello"
        val len = x?.length
    }

    // Non-null function parameter — safe call is unnecessary
    fun withNonNullParam(s: String) {
        val len = s?.length
    }

    // Non-null parameter with default value — safe call is unnecessary
    fun withDefault(s: String = "default") {
        val len = s?.length
    }
}
