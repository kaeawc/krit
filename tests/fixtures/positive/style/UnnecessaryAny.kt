package style

fun example() {
    val list = listOf(1, 2, 3)
    val result = list.filter { it > 1 }.any()
    val a = list.any { true }
    val c = list.none { true }
}
