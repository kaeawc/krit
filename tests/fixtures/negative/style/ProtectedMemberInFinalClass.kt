package style

open class Foo {
    protected val x = 1
    protected fun doWork() {}
}

sealed class Bar {
    protected abstract fun process(): String
    protected val label: String = "bar"
}
