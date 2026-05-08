package style

fun example(list: List<String>) {
    val result = list.takeIf { it.isNotEmpty() }
}
