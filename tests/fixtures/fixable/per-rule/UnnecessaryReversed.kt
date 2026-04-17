package style

fun example() {
    val list = listOf(3, 1, 2)
    val a = list.sorted().reversed()
    val b = list.sorted().asReversed()
    val c = list.sortedDescending().reversed()
    val d = list.sortedBy { it }.reversed()
    val e = list.sortedByDescending { it }.reversed()
    val f = list.reversed().sorted()
}
