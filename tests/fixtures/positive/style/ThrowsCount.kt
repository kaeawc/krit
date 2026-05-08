package fixtures.positive.style

fun riskyOperation(input: Int): String {
    if (input < 0) throw IllegalArgumentException("negative")
    if (input == 0) throw IllegalStateException("zero")
    if (input > 100) throw ArithmeticException("too large")
    return input.toString()
}
