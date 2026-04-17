package exceptions

fun process() {
    try {
        riskyOperation()
    } catch (e: Exception) {
        error(e)
    }
}

fun riskyOperation() {
    throw RuntimeException("oops")
}
