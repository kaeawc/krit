package performance

fun processListItems(list: List<Int>): List<String> {
    return list.map { it * 2 }.filter { it > 10 }.flatMap { listOf(it.toString()) }
}

fun processSetItems(values: Set<Int>): List<String> {
    return values
        .filter { it > 0 }
        .map { it * 2 }
        .distinct()
        .map { it.toString() }
}

fun processIterableItems(values: Iterable<Int>): List<Int> {
    return values
        .drop(1)
        .take(10)
        .sorted()
        .map { it + 1 }
}
