// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 68, 69, 70
// printStackTrace overloads that take arguments (the PrintStream / PrintWriter
// overloads, or a project-declared overload) print wherever the argument
// sends them, which is often a buffer rather than the console. Like Go, such a
// call counts only when its receiver is a caught exception, and a `super` call
// inside a printStackTrace override is delegation, not a console print.
package test

import java.io.PrintStream
import java.io.PrintWriter
import java.io.StringWriter

// Not reported, as in Go: the trace is captured into a string, nothing
// reaches the console.
fun Throwable.stackTraceAsString(): String {
    val sw = StringWriter()
    printStackTrace(PrintWriter(sw))
    return sw.toString()
}

fun traceOf(error: Throwable): String {
    val sw = StringWriter()
    error.printStackTrace(PrintWriter(sw, true))
    return sw.toString()
}

// Not reported, as in Go: an override that delegates to the inherited
// stream overloads before adding its own output.
class DetailedFailure(message: String) : RuntimeException(message) {
    override fun printStackTrace(s: PrintStream) {
        super.printStackTrace(s)
        s.println("details")
    }

    override fun printStackTrace(s: PrintWriter) {
        super.printStackTrace(s)
        s.println("details")
    }
}

// Not reported, in Go or here, although this one does print to the console
// instead of a logger, so the message would be true of it: neither can tell
// a console stream from a capture buffer outside a catch block, and Go never
// reports a receiver that is not the caught variable. A missed positive both
// share, not a divergence.
fun dumpToConsole(error: Throwable) {
    error.printStackTrace(PrintWriter(System.err, true))
    error.printStackTrace(System.out)
}

fun Throwable.printStackTrace(tag: String) {
    println("$tag: $message")
}

// Not reported, as in Go: a project-declared overload with arguments on a
// receiver that is not a caught exception.
fun tagged(error: Throwable) {
    error.printStackTrace("tagged")
}

// Reported, as in Go: the stream overloads and a project overload on the
// caught exception.
fun caught(work: () -> Unit) {
    try {
        work()
    } catch (e: Exception) {
        <!PrintStackTrace!>e.printStackTrace(System.err)<!>
        <!PrintStackTrace!>e.printStackTrace("caught")<!>
        <!PrintStackTrace!>e?.printStackTrace(System.out)<!>
    }
}

// Go misses this because it only compares the receiver with the nearest
// enclosing catch's variable, the inner one; `outer` is still a caught
// exception printed to the console.
fun nestedCatch(work: () -> Unit) {
    try {
        work()
    } catch (outer: Exception) {
        try {
            work()
        } catch (inner: Exception) {
            <!PrintStackTrace!>outer.printStackTrace(System.err)<!>
        }
    }
}
