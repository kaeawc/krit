// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 23, 32
package test

fun fetch(): String = "data"

fun runLater(block: () -> Unit) = block()

// Go treats every lambda as part of the block. FIR conservatively skips non-inline lambdas even when
// a helper calls one synchronously: its execution cannot be proven from the call's signature alone.
fun throwInDeferredLambda() {
    try {
        fetch()
    } finally {
        runLater { throw IllegalStateException() }
    }
}

fun throwInStoredLambda() {
    try {
        fetch()
    } finally {
        val deferred = { throw IllegalStateException() }
        println(deferred)
    }
}

fun throwInLazyInitializer() {
    try {
        fetch()
    } finally {
        val value by lazy<String> { throw IllegalStateException() }
        println("done")
    }
}

// FIR follows an inline fun literal because its execution is proven. Go skips it at its
// anonymous_function boundary, so this is an intentional FIR-only finding.
fun throwInInlineAnonymousFunction(items: List<Int>) {
    try {
        println("work")
    } finally {
        items.forEach(fun(_: Int) { <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!> })
    }
}
