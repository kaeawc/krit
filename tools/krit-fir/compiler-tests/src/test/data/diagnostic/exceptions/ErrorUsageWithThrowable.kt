// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: kotlin.error() called directly with a Throwable argument should
// trigger ErrorUsageWithThrowable.
package test

import java.io.IOException

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

fun customType(failure: Failure): Nothing = <!ErrorUsageWithThrowable!>error(failure)<!>

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
