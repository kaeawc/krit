// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: catch clauses ordered from specific to general, unrelated types,
// and clauses of different try expressions must NOT trigger
// UnreachableCatchBlock.
package test

import java.io.FileNotFoundException
import java.io.IOException

fun risky() {
    throw IOException("fail")
}

fun specificFirst() {
    try {
        risky()
    } catch (e: FileNotFoundException) {
        println(e)
    } catch (e: IOException) {
        println(e)
    } catch (e: Exception) {
        println(e)
    } catch (t: Throwable) {
        println(t)
    }
}

fun unrelatedSiblings() {
    try {
        risky()
    } catch (e: IllegalArgumentException) {
        println(e)
    } catch (e: IllegalStateException) {
        println(e)
    } catch (e: IOException) {
        println(e)
    }
}

// Error is not an Exception.
fun exceptionThenError() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } catch (e: StackOverflowError) {
        println(e)
    }
}

fun singleCatch() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } finally {
        println("done")
    }
}

// The same type caught by two different try expressions.
fun separateTries() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    }
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    }
}

// An inner try is not compared with the outer try's clauses.
fun nestedIsIndependent() {
    try {
        try {
            risky()
        } catch (e: IOException) {
            println(e)
        }
    } catch (e: Exception) {
        println(e)
    }
}

// A project exception is only shadowed by its real supertypes.
class AppFailure : RuntimeException()

fun projectException() {
    try {
        risky()
    } catch (e: IOException) {
        println(e)
    } catch (e: AppFailure) {
        println(e)
    }
}
