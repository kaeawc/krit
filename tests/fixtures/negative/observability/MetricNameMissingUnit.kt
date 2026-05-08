package test

interface Registry {
    fun counter(name: String): Counter
    fun timer(name: String): Timer
}

interface Counter {
    fun increment()
}

interface Timer {
    fun record(block: () -> Unit)
}

fun handle(registry: Registry) {
    registry.counter("requests_total").increment()
    registry.timer("request_duration_seconds").record { work() }
}

fun work() {}
