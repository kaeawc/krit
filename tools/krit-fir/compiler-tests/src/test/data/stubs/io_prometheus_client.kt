// Compiler-test source stubs; never packaged in the production artifact.
package io.prometheus.client

// simpleclient: builder methods are declared on the generic
// SimpleCollector.Builder, so `Counter.build().name(...)` resolves to
// SimpleCollector.Builder.name.
abstract class SimpleCollector<Child> {
    fun labels(vararg labelValues: String): Child = TODO()

    fun remove(vararg labelValues: String) {
        TODO()
    }

    abstract class Builder<B : Builder<B, C>, C : SimpleCollector<*>> {
        fun name(name: String): B = TODO()

        fun help(help: String): B = TODO()

        fun namespace(namespace: String): B = TODO()

        fun labelNames(vararg labelNames: String): B = TODO()

        fun register(): C = TODO()

        abstract fun create(): C
    }
}

// Java final class with static build(...) factories and instance inc();
// never constructed or subclassed by app code.
object Counter : SimpleCollector<Counter.Child>() {
    fun build(): Builder = TODO()

    fun build(name: String, help: String): Builder = TODO()

    fun inc() {
        TODO()
    }

    fun inc(amt: Double) {
        TODO()
    }

    fun get(): Double = TODO()

    class Builder : SimpleCollector.Builder<Counter.Builder, Counter>() {
        override fun create(): Counter = TODO()
    }

    class Child {
        fun inc() {
            TODO()
        }

        fun inc(amt: Double) {
            TODO()
        }
    }
}
