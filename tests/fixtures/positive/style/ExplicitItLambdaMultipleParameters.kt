package style

fun example() {
    val list = listOf("a", "b", "c")
    list.mapIndexed { index, it -> println(it.length) }
}
