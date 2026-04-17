package style

fun example() {
    val list = listOf(1, 2, 3)
    val result = list.any { it > 0 }
    val a = list.isNotEmpty()
    val b = list.isEmpty()
    val c = list.none { it > 0 }
}
