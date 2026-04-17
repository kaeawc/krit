package exceptions

fun process() {
    try {
        riskyOperation()
    } catch (e: Exception) {
        error("Something went wrong: ${e.message}")
    }
}

fun riskyOperation() {
    throw RuntimeException("oops")
}
