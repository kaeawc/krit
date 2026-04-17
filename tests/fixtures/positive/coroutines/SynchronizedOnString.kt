package test

class Cache {
    fun mutate() {
        synchronized("global") {
            println("work")
        }
    }
}
