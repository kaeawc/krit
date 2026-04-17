package fixtures.negative.potentialbugs

class AvoidReferentialEquality {
    fun check(a: String, b: String): Boolean {
        return a == b
    }
}
