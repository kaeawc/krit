package potentialbugs

class UnsafeCallOnNullableType {
    fun example(nullable: String?) {
        val len = nullable!!.length
    }
}

class PostFilterUnsafePatterns {
    data class Item(val name: String?, val age: Int?, val firstName: String?, val first: String?)

    // Checks a different field than the one accessed with !!
    fun unsafeDifferentField(list: List<Item>) {
        list.filter { it.age != null }.map { it.name!! }
    }

    // Named lambda param in filter but accessing a different field outside
    fun unsafeNamedParamDifferentField(list: List<Item>) {
        list.filter { item -> item.name != null }.flatMap { it.age!! }
    }

    // Substring collision: filter checks "firstName" but !! accesses "first"
    fun unsafeSubstringCollision(list: List<Item>) {
        list.filter { it.firstName != null }.map { it.first!! }
    }
}
