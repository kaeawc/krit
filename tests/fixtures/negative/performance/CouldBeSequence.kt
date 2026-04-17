package performance

fun processItems(list: List<Int>): List<Int> {
    return list.map { it * 2 }
}
