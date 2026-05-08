package fixtures.negative.potentialbugs

class ImplicitUnitReturnType {
    fun foo(): Unit {
        println("hi")
    }

    fun bar(): String {
        return "hello"
    }
}
