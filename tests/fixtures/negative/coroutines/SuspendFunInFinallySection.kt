package coroutines

fun doWork() {
    try {
        println("working")
    } finally {
        cleanup()
    }
}

fun cleanup() {
    println("cleaned up")
}
