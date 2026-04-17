package fixtures.negative.potentialbugs

class DoubleMutabilityForCollection {
    fun example() {
        val list = mutableListOf<String>()
        list.add("hello")
    }
}
