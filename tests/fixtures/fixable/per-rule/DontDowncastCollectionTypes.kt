package fixtures.positive.potentialbugs

class DontDowncastCollectionTypes {
    fun modify(list: List<String>) {
        val mutable = list as MutableList<String>
        mutable.add("new")
    }
}
