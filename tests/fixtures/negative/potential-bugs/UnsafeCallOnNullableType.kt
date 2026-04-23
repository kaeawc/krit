package potentialbugs

class UnsafeCallOnNullableType {
    fun example(nullable: String?) {
        val len = nullable?.length
    }

    // String literal containing !! should not trigger
    fun message() {
        val msg = "use !! to force"
    }

    // a!!.b — only a!! fires, not a!!.b as the outer node
    fun chainedAccess(a: String?) {
        // This negative fixture verifies that navigation on a postfix
        // expression is not double-reported at the navigation_expression level.
        val x = a?.length
    }
}

class PostFilterSafePatterns {
    data class Item(val name: String?, val age: Int?)

    // Same field checked in filter lambda (implicit "it")
    fun safeWithFilter(list: List<Item>): List<String?> {
        return list.filter { it.name != null }.map { it.name!! }
    }

    // filterNotNull guarantees elements are non-null
    fun safeWithFilterNotNull(list: List<String?>): List<String> {
        return list.filterNotNull().map { it!! }
    }

    // Named lambda parameter in filter, same field checked
    fun safeWithNamedParam(list: List<Item>): List<String?> {
        return list.filter { item -> item.name != null }.map { it.name!! }
    }
}
