package performance

fun processItems(list: List<Int>): List<String> {
    return list.map { it * 2 }.filter { it > 10 }.flatMap { listOf(it.toString()) }
}
