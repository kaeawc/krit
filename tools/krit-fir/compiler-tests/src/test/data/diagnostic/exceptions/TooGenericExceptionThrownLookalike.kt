// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Project classes named like the generic exceptions are not generic: neither
// Go (which skips a name the file declares a class for) nor FIR reports them.
package test

class Exception(message: String) : Throwable(message)

object Failures {
    class RuntimeException(message: String) : IllegalStateException(message)
}

fun projectException(): Nothing = throw Exception("project class")

fun nestedLookalike(): Nothing = throw Failures.RuntimeException("nested lookalike")

// A local class shadowing the name.
fun localLookalike(): Nothing {
    class Error(message: String) : Throwable(message)
    throw Error("local class")
}
