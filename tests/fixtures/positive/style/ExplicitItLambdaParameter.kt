package style

fun example() {
    val list = listOf(1, 2, 3)
    list.map { it -> it + 1 }
    val lambda = { it: Int -> it.toString() }
}
