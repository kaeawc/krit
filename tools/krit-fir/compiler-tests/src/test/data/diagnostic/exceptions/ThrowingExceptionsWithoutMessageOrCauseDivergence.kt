// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 22
package test

import java.lang.IllegalStateException as IllegalState

typealias Oops = IllegalArgumentException

object Errors {
    fun Exception(): String = "plain"
    fun RuntimeException(): String = "plain"
}

// Go reads the written name, so an alias of a listed class is not found.
fun importAlias(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>IllegalState<!>()

fun typeAlias(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>Oops<!>()

// Go reports a function that only shares the name with a listed exception.
fun functionLookalike(): String = Errors.Exception()

fun anotherFunctionLookalike(): String = Errors.RuntimeException()
