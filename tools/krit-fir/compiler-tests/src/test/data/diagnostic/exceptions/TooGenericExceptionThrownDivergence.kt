// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 19
package test

import java.lang.RuntimeException as Failure

typealias AppError = RuntimeException

class Holder {
    class Exception(message: String) : Throwable(message)
}

// Divergence (precision): Go's dispatch node is any jump expression, so it
// reports these returns as thrown generic exceptions. Nothing is thrown.
fun makeError(): Error {
    return Error("returned")
}
fun makeThrowable(): Any {
    return Throwable("returned")
}

// Divergence (recall): Go reads only the first call inside the throw, here the
// IllegalStateException constructor or the helper call; the other value the
// throw can evaluate to is a generic exception.
fun secondBranch(strict: Boolean): Nothing =
    <!TooGenericExceptionThrown!>throw<!> if (strict) IllegalStateException("strict") else Error("lenient")
fun lookup(): Throwable? = null
fun elvisAfterCall(): Nothing = <!TooGenericExceptionThrown!>throw<!> lookup() ?: RuntimeException("missing")

// Divergence (recall): Go matches the name as written, so it misses an import
// alias and a type alias of RuntimeException.
fun importAlias(): Nothing = <!TooGenericExceptionThrown!>throw<!> Failure("alias")
fun typeAlias(): Nothing = <!TooGenericExceptionThrown!>throw<!> AppError("typealias")

// Divergence (recall): the file declares a class named Exception (nested in
// Holder), so Go skips every throw of `Exception`. This one is kotlin.Exception.
fun notTheNestedClass(): Nothing = <!TooGenericExceptionThrown!>throw<!> Exception("kotlin.Exception")

// The nested class itself is not a generic exception (neither reports it).
fun nestedClass(): Nothing = throw Holder.Exception("nested")

// Divergence (recall): Go matches the caught exception by name, so a lambda
// parameter named like the catch parameter exempts the throw. The argument is
// the lambda's own value, not the caught exception.
fun shadowedCatchParameter(errors: List<Throwable>) {
    try {
        errors.size
    } catch (e: IllegalStateException) {
        errors.forEach { e -> <!TooGenericExceptionThrown!>throw<!> RuntimeException(e) }
    }
}

// Divergence (recall): Go reads only the first call, which wraps the caught
// exception; the other branch throws an unrelated generic exception.
fun wrapsInOneBranch(strict: Boolean) {
    try {
        strict.toString()
    } catch (e: IllegalStateException) {
        <!TooGenericExceptionThrown!>throw<!> if (strict) RuntimeException(e) else Error("unrelated")
    }
}
