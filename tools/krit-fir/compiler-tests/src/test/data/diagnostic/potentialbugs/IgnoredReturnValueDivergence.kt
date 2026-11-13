// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergences: Go drops these on name and receiver-spelling heuristics that exist to correct a guessed receiver
// or return type. With the callee and type resolved, the message is true of the code, so the checker reports.
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.onStart

// Go's builder-callback exemption skips an `onStart` with a lambda on a receiver named like a builder. The
// receiver here is a Flow, and `Flow.onStart` returns a new Flow that does nothing until collected.
fun builderNamedReceiver(source: Flow<Int>) {
    val requestBuilder: Flow<Int> = source
    requestBuilder.<!IgnoredReturnValue!>onStart<!> { }
}
