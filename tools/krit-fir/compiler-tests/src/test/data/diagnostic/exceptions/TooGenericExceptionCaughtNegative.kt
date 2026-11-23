// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import java.io.IOException

fun work() {}

fun specificTypes() {
    try {
        work()
    } catch (e: IOException) {
        println(e)
    } catch (e: IllegalStateException) {
        println(e)
    } catch (e: IllegalArgumentException) {
        println(e)
    } catch (e: UnsupportedOperationException) {
        println(e)
    } catch (e: NumberFormatException) {
        println(e)
    }
}

// A qualified type is not the simple name the rule matches, as in Go.
fun qualifiedTypes() {
    try {
        work()
    } catch (e: java.lang.Exception) {
        println(e)
    } catch (e: kotlin.Exception) {
        println(e)
    }
}

// A type alias of a specific exception is not generic.
typealias Failure = IOException

fun aliasOfSpecific() {
    try {
        work()
    } catch (e: Failure) {
        println(e)
    }
}

// An annotation on the type is how the rule is suppressed on a catch parameter, and the written text no
// longer equals the simple name, as in Go.
fun suppressedOnType() {
    try {
        work()
    } catch (e: @Suppress("TooGenericExceptionCaught") Exception) {
        println(e)
    }
}
