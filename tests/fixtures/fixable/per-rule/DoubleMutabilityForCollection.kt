package fixtures.positive.potentialbugs

class DoubleMutabilityForCollection {
    fun example() {
        var list = mutableListOf<String>()
        list.add("hello")
    }
}
