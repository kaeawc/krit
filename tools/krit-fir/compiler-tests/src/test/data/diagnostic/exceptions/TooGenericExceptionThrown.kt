// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 13, 16, 19, 25, 29, 33, 38, 39, 40, 42, 47, 49, 53, 55, 57, 66
package test

import java.io.IOException

class ConfigLoader {
    fun load(path: String): String {
        if (path.isEmpty()) {
            <!TooGenericExceptionThrown!>throw<!> Exception("bad path")
        }
        if (path.startsWith("/")) {
            <!TooGenericExceptionThrown!>throw<!> RuntimeException("absolute path")
        }
        if (path.endsWith(".xml")) {
            <!TooGenericExceptionThrown!>throw<!> Error("xml")
        }
        if (!path.endsWith(".yml")) {
            <!TooGenericExceptionThrown!>throw<!> Throwable("unsupported format")
        }
        return path
    }

    companion object {
        fun fail(): Nothing = <!TooGenericExceptionThrown!>throw<!> Exception("companion")
    }

    init {
        if (System.nanoTime() < 0) <!TooGenericExceptionThrown!>throw<!> Error("init")
    }

    val size: Int
        get() = <!TooGenericExceptionThrown!>throw<!> RuntimeException("getter")
}

// Qualified names, parentheses, and a throw split over two lines (reported on
// the `throw` line, like Go).
fun qualified(): Nothing = <!TooGenericExceptionThrown!>throw<!> kotlin.Exception("q")
fun javaQualified(): Nothing = <!TooGenericExceptionThrown!>throw<!> java.lang.RuntimeException("q")
fun parenthesized(): Nothing = <!TooGenericExceptionThrown!>throw<!> (Error("paren"))
fun multiLine() {
    <!TooGenericExceptionThrown!>throw<!>
        Exception("next line")
}

// The value thrown by an elvis fallback or by the first branch of an `if`.
fun elvis(cause: Throwable?): Nothing = <!TooGenericExceptionThrown!>throw<!> cause ?: Exception("fallback")
fun firstBranch(strict: Boolean): Nothing =
    <!TooGenericExceptionThrown!>throw<!> if (strict) Exception("strict") else IllegalStateException("lenient")

// Lambdas, local functions, and members of an anonymous object.
fun run(block: () -> Unit) = block()
fun inLambda() = run { <!TooGenericExceptionThrown!>throw<!> RuntimeException("lambda") }
fun inLocal() {
    fun local(): Nothing = <!TooGenericExceptionThrown!>throw<!> Throwable("local")
    val handler = object {
        fun fail(): Nothing = <!TooGenericExceptionThrown!>throw<!> Exception("anonymous")
    }
    handler.fail()
    local()
}

// A function named like the class it returns: Go reads the call's name, and
// the value thrown is an Error.
fun Error(code: Int): Error = Error("code $code")
fun factory(): Nothing = <!TooGenericExceptionThrown!>throw<!> Error(3)

// Specific exception types are fine.
fun specific(path: String) {
    if (path.isEmpty()) throw IllegalArgumentException("empty")
    if (path.isBlank()) throw IllegalStateException("blank")
    throw IOException("io")
}

// Rethrowing a caught exception, or a generic exception nested as a cause,
// constructs no generic exception at the throw.
fun rethrow() {
    try {
        specific("")
    } catch (e: Exception) {
        throw e
    }
}
fun nestedCause(): Nothing = throw IllegalStateException("wrapped", Exception("cause"))
fun viaHelper(): Nothing = throw wrap(Exception("cause"))
fun wrap(cause: Throwable): Throwable = IllegalStateException(cause)

// A subclass declared as an anonymous object is not the generic class.
fun anonymousSubclass(): Nothing = throw object : Exception("anonymous subclass") {}

// Returning or building a generic exception without throwing it.
fun build(): Exception = Exception("built")

// `also` / `apply` after the constructor: both Go (which reads the call's
// name) and FIR (which looks for the constructor call) leave these alone.
fun scoped(): Nothing = throw RuntimeException("x").also { it.printStackTrace() }
