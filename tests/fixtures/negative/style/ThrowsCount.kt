package fixtures.negative.style

fun safeOperation(input: Int): String {
    if (input < 0) throw IllegalArgumentException("negative")
    return input.toString()
}
