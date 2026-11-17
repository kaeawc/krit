// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

fun doWork() {}

class Wrapped(message: String?) : RuntimeException(message)

class WrappedCause(message: String?, cause: Throwable?) : RuntimeException(message, cause)

fun log(message: String, error: Throwable) {}

fun wrapped() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw WrappedCause(e.message, e)
    }
}

fun wrappedAsCause() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw IllegalArgumentException(e)
    }
}

fun rethrown() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw e
    }
}

fun thrownCause() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw e.cause ?: e
    }
}

fun thrownDirectMember() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw e.fillInStackTrace()
    }
}

fun logged() {
    try {
        doWork()
    } catch (e: Exception) {
        log("failed", e)
    }
}

fun safeCallIsAUse() {
    try {
        doWork()
    } catch (e: IllegalStateException) {
        throw Wrapped(e?.message)
    }
}

fun message() {
    try {
        doWork()
    } catch (e: Exception) {
        println(e.message)
    }
}

fun usedInLambda(sink: (Throwable) -> Unit) {
    try {
        doWork()
    } catch (e: Exception) {
        run { sink(e) }
    }
}

fun usedInTemplate() {
    try {
        doWork()
    } catch (e: Exception) {
        println("failed: $e")
    }
}

fun passedToWrapperThenThrown() {
    try {
        doWork()
    } catch (e: Exception) {
        val wrapper = WrappedCause("x", e)
        throw wrapper
    }
}

fun underscoreName() {
    try {
        doWork()
    } catch (_: Exception) {
        println("ignored")
    }
}

fun allowedNames() {
    try {
        doWork()
    } catch (ignored: Exception) {
        println("ignored")
    }
    try {
        doWork()
    } catch (expectedFailure: Exception) {
        println("expected")
    }
}

fun ignoredTypes() {
    try {
        doWork()
    } catch (e: InterruptedException) {
        println("interrupted")
    }
    try {
        doWork()
    } catch (e: java.net.MalformedURLException) {
        println("url")
    }
    try {
        doWork()
    } catch (e: NumberFormatException) {
        println("number")
    }
    try {
        doWork()
    } catch (e: java.text.ParseException) {
        println("parse")
    }
}

fun ignoredTypeNameInBody() {
    try {
        doWork()
    } catch (e: Exception) {
        println(NumberFormatException::class)
    }
}

fun usedInNestedHandler() {
    try {
        doWork()
    } catch (e: Exception) {
        try {
            doWork()
        } catch (inner: Exception) {
            inner.addSuppressed(e)
        }
    }
}
