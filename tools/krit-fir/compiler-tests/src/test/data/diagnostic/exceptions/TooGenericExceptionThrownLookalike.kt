// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 21
// Divergence (precision): project classes named like the generic exceptions
// are not generic. Go, like detekt, matches the written name and reports the
// unqualified ones.
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
