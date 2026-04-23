package fixtures.positive.potentialbugs

class DontDowncastCollectionTypes {
    fun modifyList(list: List<String>) {
        val mutable = list as MutableList<String>
        mutable.add("new")
    }

    fun modifySet(set: Set<String>) {
        val mutable = set as MutableSet<String>
        mutable.add("new")
    }
}
