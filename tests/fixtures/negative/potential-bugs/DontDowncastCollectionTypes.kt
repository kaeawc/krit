package fixtures.negative.potentialbugs

class DontDowncastCollectionTypes {
    fun modifyList(list: List<String>) {
        val mutable = list.toMutableList()
        mutable.add("new")
    }

    fun modifySet(set: Set<String>) {
        val mutable = set.toMutableSet()
        mutable.add("new")
    }

    @Suppress("UNCHECKED_CAST")
    fun suppressedCast(list: List<String>) {
        val mutable = list as MutableList<String>
        mutable.add("new")
    }

    fun suppressedValStatement(list: List<String>) {
        @Suppress("UNCHECKED_CAST")
        val mutable = list as MutableList<String>
        mutable.add("new")
    }
}
