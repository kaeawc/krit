// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36, 47
package test

import java.lang.RuntimeException as Failure

typealias AppError = RuntimeException

class Holder {
    class Exception(message: String) : Throwable(message)
}

// Returns of a generic exception throw nothing: neither reports them.
fun makeError(): Error {
    return Error("returned")
}
fun makeThrowable(): Any {
    return Throwable("returned")
}

// Divergence (recall): Go, like detekt, reads only a thrown call or name, not
// an `if` or elvis; the other value the throw can evaluate to is a generic
// exception.
fun secondBranch(strict: Boolean): Nothing =
    <!TooGenericExceptionThrown!>throw if (strict) IllegalStateException("strict") else Error("lenient")<!>
fun lookup(): Throwable? = null
fun elvisAfterCall(): Nothing = <!TooGenericExceptionThrown!>throw lookup() ?: RuntimeException("missing")<!>

// Divergence (recall): Go matches the name as written, so it misses an import
// alias and a type alias of RuntimeException.
fun importAlias(): Nothing = <!TooGenericExceptionThrown!>throw Failure("alias")<!>
fun typeAlias(): Nothing = <!TooGenericExceptionThrown!>throw AppError("typealias")<!>

// The file declares a nested class named Exception; this throw is still
// kotlin.Exception, and both report it.
fun notTheNestedClass(): Nothing = <!TooGenericExceptionThrown!>throw Exception("kotlin.Exception")<!>

// The nested class itself is not a generic exception (neither reports it).
fun nestedClass(): Nothing = throw Holder.Exception("nested")

// The argument is the lambda's own value, not the caught exception, so the
// catch-wrapping exemption does not apply. Go reports every such throw.
fun shadowedCatchParameter(errors: List<Throwable>) {
    try {
        errors.size
    } catch (e: IllegalStateException) {
        errors.forEach { e -> <!TooGenericExceptionThrown!>throw RuntimeException(e)<!> }
    }
}

// Divergence (recall): Go does not read an `if`; one branch wraps the caught
// exception, the other throws an unrelated generic exception.
fun wrapsInOneBranch(strict: Boolean) {
    try {
        strict.toString()
    } catch (e: IllegalStateException) {
        <!TooGenericExceptionThrown!>throw if (strict) RuntimeException(e) else Error("unrelated")<!>
    }
}
