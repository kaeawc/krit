// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 35, 44, 54, 63, 72
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

// Go finds the caught name inside the lambda, where it is a parameter of the lambda itself.
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

// Go reads an argument that starts with the caught name as the caught exception itself.
fun elvisOnMember(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException(e.message ?: "unknown")
    }
}

// Appended text adds context, so the argument is not the caught exception on its own.
fun concatenatedMember(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException(e.message + "!")
    }
}
