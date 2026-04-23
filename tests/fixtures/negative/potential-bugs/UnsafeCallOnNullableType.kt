package potentialbugs

class UnsafeCallOnNullableType {
    fun example(nullable: String?) {
        val len = nullable?.length
    }
}

class PostFilterSafePatterns {
    data class Item(val name: String?, val age: Int?)

    // Same field checked in filter lambda (implicit "it")
    fun safeWithFilter(list: List<Item>) {
        list.filter { it.name != null }.map { it.name!! }
    }

    // filterNotNull guarantees elements are non-null
    fun safeWithFilterNotNull(list: List<String?>) {
        list.filterNotNull().map { it!! }
    }

    // Named lambda parameter in filter, same field checked
    fun safeWithNamedParam(list: List<Item>) {
        list.filter { item -> item.name != null }.map { it.name!! }
    }
}
