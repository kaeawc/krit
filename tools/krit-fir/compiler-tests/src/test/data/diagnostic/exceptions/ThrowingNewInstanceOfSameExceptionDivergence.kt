// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 35, 82
package test

import java.io.IOException

typealias Failure = IOException

class Remote {
    class IOException(cause: Throwable) : Exception(cause)
}

// Go compares the written names only, so a type alias of the caught class hides the finding.
fun aliasedCaughtType(): String {
    try {
        return "ok"
    } catch (e: Failure) {
        <!ThrowingNewInstanceOfSameException!>throw IOException(e)<!>
    }
}

fun aliasedThrownType(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        <!ThrowingNewInstanceOfSameException!>throw Failure(e)<!>
    }
}

// Go reports these by spelling: the last name matches, but the thrown class is another class.
fun nestedClassWithSameName(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw Remote.IOException(e)
    }
}

// Not reported, in Go or here: inside the lambda the name is the lambda's own parameter.
fun shadowedByLambdaParameter(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        val wrap = { e: Throwable -> throw IOException(e) }
        wrap(e)
        return "x"
    }
}

fun shadowedInsideThrow(other: Throwable): String {
    try {
        return "ok"
    } catch (e: IOException) {
        return listOf(other).map { e -> throw IOException(e) }.toString()
    }
}

// Not reported, in Go or here: an elvis builds a new value from the caught exception's message.
fun elvisOnMember(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException(e.message ?: "unknown")
    }
}

// Not reported, in Go or here: appended text adds context, so the argument is not the caught exception.
fun concatenatedMember(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException(e.message + "!")
    }
}

// Reported, as in Go: the parentheses do not change the value, which is the
// caught exception.
fun parenthesizedArgument(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        <!ThrowingNewInstanceOfSameException!>throw IOException((e))<!>
    }
}
