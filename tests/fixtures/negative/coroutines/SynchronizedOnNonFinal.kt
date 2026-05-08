package test

class Worker {
    private val lock = Any()

    fun op() {
        synchronized(lock) {
            println("work")
        }
    }
}
