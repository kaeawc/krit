package test

interface Registry {
    fun counter(name: String): Counter
}

interface Counter {
    fun increment()
}

fun handle(registry: Registry) {
    registry.counter("requests").increment()
}
