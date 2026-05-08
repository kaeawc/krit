package style

fun doSomething() {
    println("hello")
}

fun doOther() = println("world")

// overridden no-op functions are allowed
override fun noOp() = Unit

fun returnsString(): String {
    return "hello"
}
