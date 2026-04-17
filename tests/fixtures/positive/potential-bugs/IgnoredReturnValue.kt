package fixtures.positive.potentialbugs

class IgnoredReturnValue {
    fun process(list: List<Int>) {
        list.map { it * 2 }
    }
}
