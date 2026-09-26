// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// Negative: an explicitly imported `error` wins over the default import of
// kotlin.error, so `error(e)` is not kotlin.error.
// Go reports this because it matches the bare name `error`; FIR is correct
// because the call resolves to the imported Failures.error.
package test

import test.Failures.error

object Failures {
    fun error(cause: Throwable): Nothing = throw IllegalStateException(cause)
}

fun importedFunction(e: Exception): Nothing = error(e)
