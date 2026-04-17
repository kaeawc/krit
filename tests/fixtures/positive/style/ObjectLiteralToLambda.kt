package style

// Java SAM interface — convertible
fun runnableExample() {
    val task: Runnable = object : Runnable {
        override fun run() {
            doWork()
        }
    }
}

fun doWork() {}

// Kotlin fun interface — convertible
fun interface Sam {
    fun execute()
}

fun funInterfaceExample() {
    val sam = object : Sam {
        override fun execute() {
            println("executing")
        }
    }
}
