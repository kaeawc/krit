package fixtures.positive.potentialbugs

class UnusedUnaryOperator {
    fun standalone(x: Int) {
        +x
    }

    fun standaloneMinus(x: Int) {
        -x
    }
}
