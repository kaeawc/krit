package fixtures.negative.emptyblocks

fun handleError() {
    try {
        riskyOperation()
    } catch (e: Exception) {
        log(e)
    }
}
