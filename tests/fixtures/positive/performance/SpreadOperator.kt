package performance

fun bar(vararg items: String) {
    println(items.joinToString())
}

// Spreading a variable causes an array copy
fun foo(args: Array<String>) {
    bar(*args)
}

// Spreading a function return causes an array copy
fun baz(items: List<String>) {
    bar(*items.toTypedArray())
}

// detekt reports spreading an array factory too.
fun arrayFactories() {
    bar(*arrayOf("a", "b"))
    takeInts(*intArrayOf(1, 2))
    bar(*arrayOfNulls<String>(2))
    bar(*emptyArray<String>())
}
