package fixtures.negative.potentialbugs

class DoubleMutabilityForCollection {
    fun example() {
        val list = mutableListOf<String>()
        list.add("hello")
    }
}

class MutableList<T>

class DoubleMutabilityForCollectionLookalike {
    fun example() {
        var list: MutableList<String> = MutableList()
    }
}
