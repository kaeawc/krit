package style

fun example() {
    val list = listOf("a", "b", "c")
    list.mapIndexed { index, item -> "$index: $item" }
}
