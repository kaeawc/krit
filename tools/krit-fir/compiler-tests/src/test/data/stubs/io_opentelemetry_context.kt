// Compiler-test source stubs; never packaged in the production artifact.
package io.opentelemetry.context

// Java interface whose close() is abstract (it narrows AutoCloseable.close to
// not throw), so `scope.use { }` and try-with-resources idioms apply.
interface Scope : AutoCloseable {
    override fun close()
}

interface ImplicitContextKeyed {
    fun makeCurrent(): Scope = TODO()

    fun storeInContext(context: Context): Context
}

interface Context {
    fun makeCurrent(): Scope = TODO()

    fun <V> get(key: ContextKey<V>): V?

    fun <V> with(key: ContextKey<V>, value: V): Context

    fun wrap(runnable: Runnable): Runnable = TODO()

    companion object {
        fun current(): Context = TODO()

        fun root(): Context = TODO()
    }
}

interface ContextKey<T> {
    companion object {
        fun <T> named(name: String): ContextKey<T> = TODO()
    }
}
