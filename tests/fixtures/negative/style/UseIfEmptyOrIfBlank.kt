package style

fun example(str: String): String {
    return str.ifEmpty { "default" }
}

fun example2(str: String): String {
    return str.ifBlank { "default" }
}
