package test

class Worker {
    private var lock = Any()

    fun op() {
        synchronized(lock) {
            println("work")
        }
    }
}
