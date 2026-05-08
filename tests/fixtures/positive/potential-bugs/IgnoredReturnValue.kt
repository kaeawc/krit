package fixtures.positive.potentialbugs

class IgnoredReturnValue {
    fun process(list: List<Int>) {
        list.map { it * 2 }
    }

    fun sequence(): Sequence<Int> = sequenceOf(1, 2, 3)

    fun processSequence() {
        sequence().map { it + 1 }
    }
}
