// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// Negative: a top-level `error` in the same package wins over the default
// import of kotlin.error, so `error(e)` is not kotlin.error.
// Go reports this because it matches the bare name `error`; FIR is correct
// because the call resolves to test.error.
package test

fun error(message: Any): Nothing = throw IllegalStateException(message.toString())

fun samePackage(e: Exception): Nothing = error(e)
