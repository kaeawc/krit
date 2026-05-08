package test

class Counter {
    private val lock = Any()

    fun work() {
        synchronized(lock) {
            println("work")
        }
    }
}
