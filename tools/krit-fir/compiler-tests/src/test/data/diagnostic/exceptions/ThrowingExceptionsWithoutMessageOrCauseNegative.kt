// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import java.io.IOException

class CustomFailure(message: String = "") : RuntimeException(message)

fun withMessage(): Nothing = throw IllegalStateException("not ready")

fun withCause(cause: Throwable): Nothing = throw RuntimeException(cause)

fun withNamedMessage(): Nothing = throw CustomFailure(message = "bad")

fun withMessageAndCause(cause: Throwable): Nothing = throw Exception("failed", cause)

fun unlisted(): Nothing = throw IOException()

fun unlistedSubclass(): Nothing = throw UnsupportedOperationException()

// A class that is not in the list.
fun projectClass(): Nothing = throw CustomFailure()

// A type name is not a call.
fun typeOnly(e: Exception): Boolean = e is IllegalStateException

fun classReference(): Any = IllegalStateException::class

fun constructorReference(): () -> Throwable = ::IllegalStateException
