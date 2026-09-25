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

// Go reports these because its resolver types a property read off a
// Throwable as a Throwable; FIR is correct because they are an array, a
// String and an array.
fun stackTraceArgument(e: Exception): Nothing = error(e.stackTrace)

fun localizedMessageArgument(e: Exception): Nothing = error(e.localizedMessage)

fun suppressedArgument(e: Exception): Nothing = error(e.suppressed)

// Go reports this because it matches the bare name `error`; FIR is correct
// because the call resolves to the `with` receiver's member, not kotlin.error.
class Log {
    fun error(message: Any?) {
        println(message)
    }
}

fun withReceiver(log: Log, e: Exception) {
    with(log) { error(e) }
}

// A local `var` captured and reassigned by a lambda has no smart cast; FIR
// only narrows it from an `is` check that proves a Throwable on this path.
fun capturedVarUnchecked(initial: Any): Nothing {
    var value = initial
    val reset = { value = "reset" }
    reset()
    error(value)
}

fun capturedVarOtherType(initial: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value is CharSequence) error(value)
}

// Go reports this because it narrows from an `is` check anywhere in an `||`
// condition; FIR is correct because `enabled` alone can take the branch.
fun capturedVarDisjunction(initial: Any, enabled: Boolean) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (enabled || value is Exception) error(value)
}

// Go reports this because it narrows `value` in every branch body of the
// `if`, including `else`; FIR is correct because the else branch runs only
// when `value` is not an Exception.
fun capturedVarElseBranch(initial: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value is Exception) println("exception") else error(value)
}

// Throwing directly is the recommended form.
fun rethrow(e: Exception): Nothing = throw IllegalStateException("wrapped", e)

// Reassigned after the check: the value passed is no longer the checked
// Throwable, so neither the early exit nor the `is` branch proves it.
fun capturedVarReassignedAfterGuard(initial: Any, replacement: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value !is Exception) return
    value = replacement
    error(value)
}

fun capturedVarReassignedInBranch(initial: Any, replacement: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value is Exception) {
        value = replacement
        error(value)
    }
}
