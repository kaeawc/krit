package fixtures.positive.potentialbugs

class AvoidReferentialEquality {
    fun check(a: String, b: String): Boolean {
        return a === b
    }

    fun literal(candidate: Any): Boolean {
        return candidate !== "a"
    }

    fun local(candidate: Any): Boolean {
        val text: String = "a"
        return text === candidate
    }
}
