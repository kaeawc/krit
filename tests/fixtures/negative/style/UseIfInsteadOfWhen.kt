package style

fun example(x: Int): String {
    return when {
        x > 0 -> "a"
        x < 0 -> "b"
        else -> "c"
    }
}
