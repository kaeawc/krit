package style

fun example() {
    val lists = listOf(listOf(1, 2), listOf(3))
    val count = lists.sumOf { it.size }
}
