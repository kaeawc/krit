package fixtures.positive.potentialbugs

class UnusedUnaryOperator {
    fun standalone(x: Int) {
        +x
    }

    fun standaloneMinus(x: Int) {
        -x
    }

    fun multilineUnused() {
        val x = 1 + 2
        + 3
    }

    fun multilineUnusedBinary() {
        val x = 1 + 2
        + 3 + 4 + 5
    }
}
