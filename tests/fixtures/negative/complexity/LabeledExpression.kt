package complexity

fun processItems(items: List<String>) {
    for (item in items) {
        if (item.isEmpty()) return
        println(item)
    }
}
