// Compiler-test source stubs; never packaged in the production artifact.
package io.micrometer.core.instrument

import io.micrometer.core.instrument.composite.CompositeMeterRegistry

interface Meter

abstract class MeterRegistry {
    fun counter(name: String, vararg tags: String): Counter = TODO()

    fun timer(name: String, vararg tags: String): Timer = TODO()

    fun <T : Number> gauge(name: String, number: T): T? = TODO()
}

// Java class of statics over the global composite registry.
object Metrics {
    @JvmField
    val globalRegistry: CompositeMeterRegistry = TODO()

    fun counter(name: String, vararg tags: String): Counter = TODO()

    fun timer(name: String, vararg tags: String): Timer = TODO()
}

interface Counter : Meter {
    fun increment() {
        TODO()
    }

    fun increment(amount: Double)

    fun count(): Double

    // Java interface static `Counter.builder(name)`; callable id gains
    // `.Companion` (see README).
    companion object {
        fun builder(name: String): Builder = TODO()
    }

    class Builder {
        fun tag(key: String, value: String): Builder = TODO()

        fun tags(vararg tags: String): Builder = TODO()

        fun description(description: String?): Builder = TODO()

        fun baseUnit(unit: String?): Builder = TODO()

        fun register(registry: MeterRegistry): Counter = TODO()
    }
}

interface Timer : Meter {
    fun record(f: Runnable)

    fun <T> recordCallable(f: java.util.concurrent.Callable<T>): T?

    companion object {
        fun builder(name: String): Builder = TODO()
    }

    class Builder {
        fun tag(key: String, value: String): Builder = TODO()

        fun register(registry: MeterRegistry): Timer = TODO()
    }
}
