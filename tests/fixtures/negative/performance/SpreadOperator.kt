package performance

fun bar(vararg items: String) {
    println(items.joinToString())
}

fun takeInts(vararg items: Int) {
    println(items.joinToString())
}

// Spreading a variable causes an array copy
fun foo(args: Array<String>) {
    args.forEach { bar(it) }
}

// Spreading a function return causes an array copy
fun baz(items: List<String>) {
    items.forEach { bar(it) }
}

fun arrayFactories() {
    bar(*arrayOf("a", "b"))
    takeInts(*intArrayOf(1, 2))
    bar(*arrayOfNulls<String>(2))
    bar(*emptyArray<String>())
}

fun forward(vararg items: String) {
    bar(*items)
}
