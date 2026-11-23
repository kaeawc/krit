// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 29, 44x2
package finallyscopes

inline fun invokeInline(noinline deferred: () -> Unit, immediate: () -> Unit) {
    deferred()
    immediate()
}

// A noinline parameter can run synchronously, but its signature does not prove this.
// FIR abstains even though Go reports; the callee body must not be read to decide.
fun noinlineArgument() {
    try {
        println("work")
    } finally {
        invokeInline({ throw IllegalStateException() }, { println("cleanup") })
    }
}

// The inner finally executes within its own function regardless of the outer deferred boundary.
fun finallyInsideLocalFunction() {
    try {
        println("work")
    } finally {
        fun local() {
            try {
                println("local work")
            } finally {
                <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!>
            }
        }
        local()
    }
}

fun finallyInsideStoredLambda(): () -> Unit {
    try {
        println("work")
    } finally {
        return {
            try {
                println("deferred work")
            } finally {
                <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!>
            }
        }
    }
}
