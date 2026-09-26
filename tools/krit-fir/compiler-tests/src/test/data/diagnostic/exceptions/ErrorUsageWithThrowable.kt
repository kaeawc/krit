// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 25, 28, 31, 48, 54, 58, 68, 77, 85, 95, 104
// Positive: kotlin.error() called directly with a Throwable argument should
// trigger ErrorUsageWithThrowable.
package test

import java.io.IOException
import java.lang.reflect.InvocationTargetException

fun riskyOperation() {
    throw RuntimeException("oops")
}

fun caught() {
    try {
        riskyOperation()
    } catch (e: Exception) {
        <!ErrorUsageWithThrowable!>error(e)<!>
    }
}

fun runtimeSubtype(e: RuntimeException): Nothing = <!ErrorUsageWithThrowable!>error(e)<!>

// Java exception type from the JDK.
fun javaException(ex: IOException): Nothing = <!ErrorUsageWithThrowable!>error(ex)<!>

// A freshly constructed exception is still a Throwable argument.
fun constructed(): Nothing = <!ErrorUsageWithThrowable!>error(IllegalStateException("boom"))<!>

// Named argument: the message names the argument expression, not the label.
fun named(cause: Exception): Nothing = <!ErrorUsageWithThrowable!>error(message = cause)<!>

class DomainException(message: String) : Exception(message)

typealias Failure = DomainException

// Deliberate FIR recall addition: Go does not report this because its source
// inference does not expand the typealias; FIR resolves it to a Throwable.
fun customType(failure: Failure): Nothing = <!ErrorUsageWithThrowable!>error(failure)<!>

// Deliberate FIR recall addition: Go does not report this because its source
// inference does not use the type parameter's bound; FIR resolves it.
fun <T : Throwable> generic(t: T): Nothing = <!ErrorUsageWithThrowable!>error(t)<!>

// Smart cast to Exception: the value passed is a Throwable.
fun smartCast(value: Any) {
    if (value is Exception) {
        <!ErrorUsageWithThrowable!>error(value)<!>
    }
}

// Inside a lambda the call is still kotlin.error.
fun inLambda(errors: List<Exception>) {
    errors.forEach { <!ErrorUsageWithThrowable!>error(it)<!> }
}

fun multiLine(e: Exception) {
    <!ErrorUsageWithThrowable!>error<!>(
        e,
    )
}

// Unstable smart cast on a `var` property: FIR keeps the declared type `Any`
// on the argument, but the `is` check still proves an Exception is passed.
// Go reports this, so FIR must too.
class Holder(var value: Any) {
    fun check() {
        if (value is Exception) <!ErrorUsageWithThrowable!>error(value)<!>
    }
}

// Unstable smart cast on a local `var` captured and reassigned by a lambda.
fun capturedVar(initial: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value is Exception) <!ErrorUsageWithThrowable!>error(value)<!>
}

fun capturedVarWhen(initial: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    when (value) {
        is Exception -> <!ErrorUsageWithThrowable!>error(value)<!>
        else -> Unit
    }
}

fun capturedVarConjunction(initial: Any, enabled: Boolean) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (enabled && value is Exception) {
        <!ErrorUsageWithThrowable!>error(value)<!>
    }
}

fun capturedVarEarlyExit(initial: Any) {
    var value = initial
    val reset = { value = "reset" }
    reset()
    if (value !is Exception) return
    <!ErrorUsageWithThrowable!>error(value)<!>
}

// Deliberate FIR recall additions below: Go does not report these because its
// source inference cannot type the argument expression (a `!!`, Elvis, `when`,
// Java getter or generic call result); FIR resolves each one to a Throwable.
fun notNullAssertion(t: Throwable?): Nothing = <!ErrorUsageWithThrowable!>error(t!!)<!>

fun causeNotNull(e: Exception): Nothing = <!ErrorUsageWithThrowable!>error(e.cause!!)<!>

fun causeOrSelf(e: Exception): Nothing = <!ErrorUsageWithThrowable!>error(e.cause ?: e)<!>

fun elvisReturn(e: Exception?) {
    <!ErrorUsageWithThrowable!>error(e ?: return)<!>
}

fun whenArgument(e: Exception): Nothing = <!ErrorUsageWithThrowable!>error(when { e.message == null -> e; else -> e })<!>

fun javaGetter(ite: InvocationTargetException): Nothing = <!ErrorUsageWithThrowable!>error(ite.targetException)<!>

fun checkNotNullArgument(e: Exception?): Nothing = <!ErrorUsageWithThrowable!>error(checkNotNull(e))<!>

fun requireNotNullCause(e: Exception): Nothing = <!ErrorUsageWithThrowable!>error(requireNotNull(e.cause))<!>

fun lambdaResult(block: () -> Throwable): Nothing = <!ErrorUsageWithThrowable!>error(block())<!>
