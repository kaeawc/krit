package complexity

fun processItems(items: List<String>) {
    items.forEach {
        if (it.isEmpty()) return@forEach
        println(it)
    }
}
