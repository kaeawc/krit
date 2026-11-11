// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import java.io.IOException

fun otherValue(other: Throwable): String {
    try {
        return "ok"
    } catch (e: Exception) {
        val cause = e.cause
        if (other is IOException || cause is IOException || e.cause is IOException) return "io"
        val safe = e as? IOException
        return when (e) {
            is IllegalStateException -> "state"
            else -> safe.toString()
        }
    }
}

fun outsideCatch(e: Exception): Boolean = e is IOException

fun specificCatch(): String {
    try {
        return "ok"
    } catch (e: IOException) {
        return "io"
    }
}
