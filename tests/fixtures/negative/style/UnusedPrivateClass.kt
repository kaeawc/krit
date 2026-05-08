package fixtures.negative.style

private class Used {
    fun doWork() {
        println("working")
    }
}

fun main() {
    val x = Used()
    x.doWork()
}
