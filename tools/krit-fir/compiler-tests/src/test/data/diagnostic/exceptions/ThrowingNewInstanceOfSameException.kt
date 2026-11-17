// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 19, 29, 38, 48, 60
package test

import java.io.IOException

fun wrapsCaught(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        <!ThrowingNewInstanceOfSameException!>throw IOException(e)<!>
    }
}

fun wrapsCause(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        <!ThrowingNewInstanceOfSameException!>throw IOException(e.cause)<!>
    }
}

class Failure(cause: Throwable) : Exception(cause)

fun wrapsNamed(): String {
    try {
        return "ok"
    } catch (e: Failure) {
        <!ThrowingNewInstanceOfSameException!>throw Failure(cause = e)<!>
    }
}

fun insideBranch(flag: Boolean): String {
    try {
        return "ok"
    } catch (e: IOException) {
        if (flag) {
            <!ThrowingNewInstanceOfSameException!>throw IOException(e)<!>
        }
        return "other"
    }
}

fun insideLambda(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        return listOf(1).map { <!ThrowingNewInstanceOfSameException!>throw IOException(e)<!> }.toString()
    }
}

// An outer catch parameter read inside a nested catch is still a caught exception.
fun nestedCatch(): String {
    try {
        return "ok"
    } catch (outer: IOException) {
        try {
            return "inner"
        } catch (inner: IllegalStateException) {
            <!ThrowingNewInstanceOfSameException!>throw IOException(outer)<!>
        }
    }
}
