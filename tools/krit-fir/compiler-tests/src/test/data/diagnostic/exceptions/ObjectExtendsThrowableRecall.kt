// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses. Go matches only a direct supertype named
// Throwable, Exception, Error, or RuntimeException, and never visits a
// companion object. Each of these is a singleton that is a Throwable.
package test

import java.io.IOException
import java.lang.IllegalArgumentException as BadArgument

// Go misses this because IllegalStateException is not one of its four names.
<!ObjectExtendsThrowable!>object<!> NotReady : IllegalStateException("not ready")

// Go misses this because IOException is not one of its four names.
<!ObjectExtendsThrowable!>object<!> Disconnected : IOException("disconnected")

// Go misses this because StackOverflowError is not one of its four names.
<!ObjectExtendsThrowable!>object<!> TooDeep : StackOverflowError()

open class AppFailure(message: String) : RuntimeException(message)

// Go misses this because it reads only the direct supertype's name.
<!ObjectExtendsThrowable!>object<!> Timeout : AppFailure("timeout")

typealias Failure = Exception

// Go misses this because it does not expand the type alias.
<!ObjectExtendsThrowable!>object<!> Aliased : Failure()

// Go misses this because it does not follow the import alias.
<!ObjectExtendsThrowable!>object<!> ImportAliased : BadArgument()

class Service {
    // Go misses this because it never visits companion objects.
    <!ObjectExtendsThrowable!>companion<!> object : Exception("companion")
}

class NamedCompanion {
    // Go misses this because it never visits companion objects.
    <!ObjectExtendsThrowable!>companion<!> object Failed : RuntimeException("failed")
}

// Go misses these because it looks the object up by simple name, and a class
// with the same name declared later in the file replaces it, so Go reads
// that class's supertypes instead of the object's.
<!ObjectExtendsThrowable!>object<!> Rejected : Exception()

class RejectionHolder {
    class Rejected
}

class Scheduler {
    <!ObjectExtendsThrowable!>object<!> Busy : RuntimeException()
}

class Busy : Comparable<Busy> {
    override fun compareTo(other: Busy): Int = 0
}
