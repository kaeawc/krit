package fixtures.negative.potentialbugs

class DontDowncastCollectionTypes {
    fun modify(list: List<String>) {
        val mutable = list.toMutableList()
        mutable.add("new")
    }
}
