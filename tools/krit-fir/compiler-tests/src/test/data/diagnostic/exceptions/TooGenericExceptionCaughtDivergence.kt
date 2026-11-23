// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 54
package test

import java.lang.RuntimeException as Boom

typealias Failure = Exception

typealias Anything = Throwable

object Local {
    class Exception(message: String) : kotlin.Exception(message)

    fun nestedClassWithTheSameName() {
        try {
            work()
        } catch (e: Exception) {
            println(e)
        }
    }
}

class Error(message: String) : kotlin.Exception(message)

fun work() {}

// Resolution proves Failure/Anything are canonical generic classes; Go misses their written aliases.
fun aliasOfGeneric() {
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Failure) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Anything) {
        println(e)
    }
}

// Resolution proves Boom is java.lang.RuntimeException; Go only compares the written import alias.
fun importAlias() {
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Boom) {
        println(e)
    }
}

// Go reports a project class that only shares the simple name with a generic exception.
fun projectClassNamedError() {
    try {
        work()
    } catch (e: Error) {
        println(e)
    }
}
