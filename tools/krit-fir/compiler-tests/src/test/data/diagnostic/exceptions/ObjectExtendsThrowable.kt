// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 18, 20, 22, 24, 28, 30, 34, 36, 41, 45, 50, 53
// Named objects that extend Throwable directly, the shapes Go reports: the
// finding sits on the declaration's first line (its modifier list, else
// `object`), like Go.
package test

import java.io.Serializable

<!ObjectExtendsThrowable!>object<!> SingletonException : Exception("singleton error")

<!ObjectExtendsThrowable!>object<!> SingletonRuntime : RuntimeException()

<!ObjectExtendsThrowable!>object<!> SingletonThrowable : Throwable()

<!ObjectExtendsThrowable!>object<!> SingletonError : Error("fatal")

<!ObjectExtendsThrowable!>object<!> QualifiedJava : java.lang.RuntimeException()

<!ObjectExtendsThrowable!>object<!> QualifiedKotlin : kotlin.Error()

<!ObjectExtendsThrowable!>object<!> WithInterface : Serializable, Exception("with interface")

<!ObjectExtendsThrowable!>object<!> WithBody : Exception("with body") {
    private fun readResolve(): Any = WithBody
}

<!ObjectExtendsThrowable!>data<!> object DataSingleton : Exception()

<!ObjectExtendsThrowable!>@Suppress("unused")<!>
object Annotated : Exception()

/** KDoc is not part of the declaration's first line, in Go or here. */
<!ObjectExtendsThrowable!>object<!> Documented : Exception()

<!ObjectExtendsThrowable!>internal<!> object
    SplitHeader : Exception()

// K2 places an object's diagnostics at `object`; this one is moved to the
// first modifier, where Go reports.
<!ObjectExtendsThrowable!>internal<!>
object ModifierAbove : Exception()

/** KDoc before annotations. */
<!ObjectExtendsThrowable!>@Suppress("unused")<!>
@Deprecated("old")
private object StackedAnnotations : Exception()

class Outer {
    <!ObjectExtendsThrowable!>object<!> Nested : RuntimeException()

    interface Inner {
        <!ObjectExtendsThrowable!>object<!> Deep : Throwable()
    }
}

// --- Negatives ---

class ClassBased : Exception("class-based error")

open class OpenBase : RuntimeException()

object Utility {
    fun doStuff() {}
}

interface Marker

object ImplementsMarker : Marker

// An object expression makes a new instance on each evaluation, so it is not
// a singleton; Go does not visit object literals either.
val anonymous = object : Exception("anonymous") {}

fun localDeclarations(): Throwable {
    class LocalException : Exception()
    val literal = object : RuntimeException() {}
    return if (literal.message == null) LocalException() else literal
}
