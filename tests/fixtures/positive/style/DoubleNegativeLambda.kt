package style

fun example(list: List<String>) {
    val result = list.filterNot { !it.startsWith("a") }
}
