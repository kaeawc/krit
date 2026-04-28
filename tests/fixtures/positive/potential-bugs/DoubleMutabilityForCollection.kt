package fixtures.positive.potentialbugs

class DoubleMutabilityForCollection {
    fun example() {
        var list = mutableListOf<String>()
        list.add("hello")

        var explicit: MutableSet<String> = mutableSetOf()
        explicit.add("world")
    }
}
