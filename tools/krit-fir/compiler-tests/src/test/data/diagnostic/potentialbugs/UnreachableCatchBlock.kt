// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 32, 42, 52, 63, 79x2, 81, 92, 103, 117, 129, 144, 158
// Positive: a catch clause whose type an earlier clause of the same try
// already catches (the same class, or a supertype) triggers
// UnreachableCatchBlock on the later clause's `catch` line.
package test

import java.io.FileNotFoundException
import java.io.IOException
import java.io.InterruptedIOException
import java.net.SocketTimeoutException

fun risky() {
    throw IOException("fail")
}

fun subtypeAfterSupertype() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IOException) {
        println(e)
    }
}

fun socketTimeoutAfterInterruptedIO() {
    try {
        risky()
    } catch (e: InterruptedIOException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: SocketTimeoutException) {
        println(e)
    }
}

fun duplicate() {
    try {
        risky()
    } catch (e: IllegalStateException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IllegalStateException) {
        println(e)
    }
}

fun throwableCatchesError() {
    try {
        risky()
    } catch (t: Throwable) {
        println(t)
    } <!UnreachableCatchBlock!>catch<!> (e: StackOverflowError) {
        println(e)
    }
}

// Transitive: FileNotFoundException -> IOException -> Exception.
fun transitive() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: FileNotFoundException) {
        println(e)
    }
}

// Every earlier clause is compared, not only the one directly above. Go
// reports once per shadowing clause: the IllegalArgumentException clause
// twice (RuntimeException and Exception both catch it) and the IOException
// clause once (only Exception does).
fun shadowedByTwo() {
    try {
        risky()
    } catch (e: RuntimeException) {
        println(e)
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IllegalArgumentException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IOException) {
        println(e)
    }
}

// Kotlin exception types are type aliases of the JDK classes.
fun kotlinAliases() {
    try {
        risky()
    } catch (e: RuntimeException) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: NumberFormatException) {
        println(e)
    }
}

// A try used as an expression, inside a lambda.
fun expressionInLambda(): () -> Int = {
    try {
        42
    } catch (e: Exception) {
        0
    } <!UnreachableCatchBlock!>catch<!> (e: ArithmeticException) {
        1
    }
}

// A nested try inside a catch body is checked on its own.
fun nested() {
    try {
        risky()
    } catch (e: IOException) {
        try {
            risky()
        } catch (inner: Throwable) {
            println(inner)
        } <!UnreachableCatchBlock!>catch<!> (inner: IOException) {
            println(inner)
        }
    }
}

// A clause whose parameter spans lines reports on the `catch` line.
fun multiLineClause() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (
        e: IOException
    ) {
        println(e)
    }
}

// A member of an object expression.
fun anonymous(): Any {
    return object {
        fun run() {
            try {
                risky()
            } catch (e: IOException) {
                println(e)
            } <!UnreachableCatchBlock!>catch<!> (e: FileNotFoundException) {
                println(e)
            }
        }
    }
}

class Service {
    companion object {
        fun load() {
            try {
                risky()
            } catch (e: Exception) {
                println(e)
            } <!UnreachableCatchBlock!>catch<!> (e: UnsupportedOperationException) {
                println(e)
            } finally {
                println("done")
            }
        }
    }
}
