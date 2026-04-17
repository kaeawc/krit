package performance

fun bar(vararg items: String) {
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
