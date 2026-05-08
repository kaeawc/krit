package style

fun example() {
    val list = listOf(1, 2, 3)
    list.map { it + 1 }
    val lambda = { value: Int -> value.toString() }
    list.flatMap { it }
}
