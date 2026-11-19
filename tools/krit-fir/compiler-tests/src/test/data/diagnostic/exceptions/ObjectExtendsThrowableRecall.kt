// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 15, 35, 41, 46, 51, 58
// Singletons that are Throwables. Go matches the superclass by simple name
// against a fixed list of Throwable classes, so it misses the ones marked
// below; the rest it reports too.
package test

import java.io.IOException
import java.lang.IllegalArgumentException as BadArgument

// Reported, as in Go: IllegalStateException is in its list.
object <!ObjectExtendsThrowable!>NotReady<!> : IllegalStateException("not ready")

// Reported, as in Go: IOException is in its list.
object <!ObjectExtendsThrowable!>Disconnected<!> : IOException("disconnected")

// Go misses this because StackOverflowError is not in its list.
object <!ObjectExtendsThrowable!>TooDeep<!> : StackOverflowError()

open class AppFailure(message: String) : RuntimeException(message)

// Go misses this because it reads only the superclass's own name.
object <!ObjectExtendsThrowable!>Timeout<!> : AppFailure("timeout")

typealias Failure = Exception

// Go misses this because it does not expand the type alias.
object <!ObjectExtendsThrowable!>Aliased<!> : Failure()

// Go misses this because it does not follow the import alias.
object <!ObjectExtendsThrowable!>ImportAliased<!> : BadArgument()

class Service {
    // Reported, as in Go: a companion object is a singleton too.
    <!ObjectExtendsThrowable!>companion<!> object : Exception("companion")
}

class AnnotatedCompanion {
    // Reported on `companion`, as in Go, not on the annotation above it.
    @Suppress("unused")
    <!ObjectExtendsThrowable!>companion<!> object : Exception("annotated")
}

class NamedCompanion {
    // Reported, as in Go.
    companion object <!ObjectExtendsThrowable!>Failed<!> : RuntimeException("failed")
}

// Reported, as in Go: a class with the same name declared later in the file
// does not replace the object's own supertypes.
object <!ObjectExtendsThrowable!>Rejected<!> : Exception()

class RejectionHolder {
    class Rejected
}

class Scheduler {
    object <!ObjectExtendsThrowable!>Busy<!> : RuntimeException()
}

class Busy : Comparable<Busy> {
    override fun compareTo(other: Busy): Int = 0
}
