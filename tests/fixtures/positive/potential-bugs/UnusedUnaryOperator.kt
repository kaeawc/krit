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

    fun unusedInFinally(): Int {
        val x: Int = try {
            something()
        } finally {
            -1
        }
        return x
    }

    fun unusedInUnitLambda(items: List<Int>) {
        items.forEach {
            println(it)
            -it
        }
    }

    fun unusedComparison(x: Int) {
        val a = x
        -x < 2
    }
}
