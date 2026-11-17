// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import java.io.FileNotFoundException
import java.io.IOException

class CustomException(message: String, cause: Throwable) : RuntimeException(message, cause)

fun differentType(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw CustomException("failed", e)
    }
}

fun subtypeIsAnotherClass(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw FileNotFoundException(e.message)
    }
}

fun addsMessage(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException("context", e)
    }
}

fun unrelatedArgument(message: String): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException(message)
    }
}

fun plainRethrow(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw e
    }
}

fun noArguments(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        throw IOException()
    }
}

fun notInCatch(cause: Throwable): String {
    throw IOException(cause)
}

fun underscoreParameter(): String {
    try {
        return "ok"
    } catch (_: IOException) {
        throw IOException()
    }
}

fun tryBodyThrow(cause: Throwable): String {
    try {
        throw IOException(cause)
    } catch (e: IllegalStateException) {
        return "x"
    }
}

fun thrownInsideCatchOfOtherType(): String {
    try {
        return "ok"
    } catch (e: IllegalStateException) {
        throw IOException(e)
    }
}
