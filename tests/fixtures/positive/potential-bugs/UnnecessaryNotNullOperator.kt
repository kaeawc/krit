package fixtures.positive.potentialbugs

class UnnecessaryNotNullOperator {
    fun example() {
        val x: String = "hello"
        val y = x!!
    }

    fun resolvedType(value: KnownThing) {
        println(value!!)
    }
}

class KnownThing
