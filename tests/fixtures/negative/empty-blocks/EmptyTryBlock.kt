package fixtures.negative.emptyblocks

fun attempt() {
    try {
        riskyOp()
    } catch (e: Exception) {
        log(e)
    }
}
