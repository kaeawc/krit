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
