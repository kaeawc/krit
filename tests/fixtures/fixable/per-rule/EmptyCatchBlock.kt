package fixtures.positive.emptyblocks

fun handleError() {
    try {
        riskyOperation()
    } catch (e: Exception) { }
}
