package style

fun example(items: List<String>) {
    items.forEach { item ->
        println(item)
        println(item.length)
        println(item.uppercase())
    }
}
