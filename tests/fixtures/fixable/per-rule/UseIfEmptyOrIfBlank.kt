package style

fun example(str: String): String {
    return if (str.isEmpty()) "default" else str
}

fun example2(str: String): String {
    return if (str.isBlank()) "default" else str
}

fun example3(list: List<Int>): List<Int> {
    return if (list.isEmpty()) listOf(1) else list
}

fun example4(list: List<Int>): List<Int> {
    return if (list.isNotEmpty()) list else listOf(2)
}

fun example5(str: String): String {
    return if (str.isNotBlank()) str else "bar"
}
