// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 23, 28, 31
// Go's exemption: a generic exception that wraps the exception caught by the
// nearest enclosing catch, passed directly as an argument.
package test

import java.io.IOException

fun io() {}

fun wrapsCaught() {
    try { io() } catch (e: IOException) { throw RuntimeException(e) }
    try { io() } catch (e: IOException) { throw RuntimeException("context", e) }
    // Named arguments need a Kotlin constructor: kotlin.Throwable.
    try { io() } catch (e: IOException) { throw Throwable(cause = e, message = "named") }
    // The nearest catch encloses the lambda, as in Go.
    try { io() } catch (e: IOException) { listOf(1).forEach { throw RuntimeException(e) } }
}

fun doesNotWrapCaught(outside: Throwable) {
    // Not the caught exception itself.
    try { io() } catch (e: IOException) { <!TooGenericExceptionThrown!>throw<!> RuntimeException("cause", e.cause) }
    try { io() } catch (e: IOException) { <!TooGenericExceptionThrown!>throw<!> RuntimeException(e.message) }
    // Only the nearest catch counts: `e` belongs to the outer one.
    try {
        io()
    } catch (e: IOException) {
        try { io() } catch (inner: IllegalStateException) { <!TooGenericExceptionThrown!>throw<!> RuntimeException(e) }
    }
    // Outside any catch, a Throwable argument is no exemption.
    <!TooGenericExceptionThrown!>throw<!> RuntimeException(outside)
}

fun failing(): Nothing = error("failing")

// The value of a thrown try expression. Its catch block is the nearest catch
// of the value it produces: wrapping that catch's parameter is exempt, as in
// `catch (e) { throw RuntimeException(e) }`. Go reads only the first call
// inside the throw, failing(), and reports neither.
fun thrownTryWrapsCaught(): Nothing =
    throw try { failing() } catch (e: IllegalStateException) { RuntimeException(e) }

// Divergence (recall): this catch block's value drops the caught exception,
// and Go, reading only failing(), misses it.
fun thrownTryDropsCaught(): Nothing =
    <!TooGenericExceptionThrown!>throw<!> try { failing() } catch (e: IllegalStateException) { RuntimeException("dropped") }
