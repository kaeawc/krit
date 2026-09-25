// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: error() with a message, a non-Throwable value, or a call that does
// not resolve to kotlin.error must NOT trigger ErrorUsageWithThrowable.
package test

fun literal(): Nothing = error("something went wrong")

fun template(e: Exception): Nothing = error("Something went wrong: ${e.message}")

fun messageProperty(e: Exception): Nothing = error(e.toString())

// A String parameter named `error` is not a Throwable.
fun stringNamedError(error: String): Nothing = error(error)

fun anyValue(value: Any): Nothing = error(value)

fun nothingArgument(): Nothing = error(TODO())

// Go only matches the bare `error(...)` form; the qualified call is left alone.
fun qualified(e: Exception): Nothing = kotlin.error(e)

// Local lookalikes: a member and a local function named `error`.
class Logger {
    fun error(t: Throwable) {
        println(t)
    }

    fun log(t: Throwable) {
        error(t)
    }
}

fun loggerCall(logger: Logger, t: Throwable) {
    logger.error(t)
}

fun localFunction(t: Throwable) {
    fun error(x: Throwable) {
        println(x)
    }
    error(t)
}

// Throwing directly is the recommended form.
fun rethrow(e: Exception): Nothing = throw IllegalStateException("wrapped", e)
