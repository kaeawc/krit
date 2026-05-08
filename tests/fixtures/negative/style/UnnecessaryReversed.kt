package style

fun example() {
    val list = listOf(3, 1, 2)
    val a = list.sortedDescending()
    val b = list.sorted()
    val c = list.reversed()
    val d = list.sorted().map { it * 2 }
    val e = list.reversed().map { it * 2 }
}
