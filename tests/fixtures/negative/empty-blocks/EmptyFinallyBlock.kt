package fixtures.negative.emptyblocks

fun doWork() {
    try {
        riskyOperation()
    } finally {
        cleanup()
    }
}
