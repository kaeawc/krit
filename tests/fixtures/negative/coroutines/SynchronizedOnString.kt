package test

class Cache {
    private val lock = Any()

    fun mutate() {
        synchronized(lock) {
            println("work")
        }
    }
}
