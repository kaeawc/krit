// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 18, 20, 22, 24, 28, 31, 34, 37, 41, 46, 49, 52
// Named objects that extend Throwable directly, the shapes Go reports: the
// finding sits on the object's name, like Go, whatever modifiers and
// annotations come before it.
package test

import java.io.Serializable

object <!ObjectExtendsThrowable!>SingletonException<!> : Exception("singleton error")

object <!ObjectExtendsThrowable!>SingletonRuntime<!> : RuntimeException()

object <!ObjectExtendsThrowable!>SingletonThrowable<!> : Throwable()

object <!ObjectExtendsThrowable!>SingletonError<!> : Error("fatal")

object <!ObjectExtendsThrowable!>QualifiedJava<!> : java.lang.RuntimeException()

object <!ObjectExtendsThrowable!>QualifiedKotlin<!> : kotlin.Error()

object <!ObjectExtendsThrowable!>WithInterface<!> : Serializable, Exception("with interface")

object <!ObjectExtendsThrowable!>WithBody<!> : Exception("with body") {
    private fun readResolve(): Any = WithBody
}

data object <!ObjectExtendsThrowable!>DataSingleton<!> : Exception()

@Suppress("unused")
object <!ObjectExtendsThrowable!>Annotated<!> : Exception()

/** KDoc does not move the finding, in Go or here. */
object <!ObjectExtendsThrowable!>Documented<!> : Exception()

internal object
    <!ObjectExtendsThrowable!>SplitHeader<!> : Exception()

// A modifier on its own line does not move the finding.
internal
object <!ObjectExtendsThrowable!>ModifierAbove<!> : Exception()

/** KDoc before annotations. */
@Suppress("unused")
@Deprecated("old")
private object <!ObjectExtendsThrowable!>StackedAnnotations<!> : Exception()

class Outer {
    object <!ObjectExtendsThrowable!>Nested<!> : RuntimeException()

    interface Inner {
        object <!ObjectExtendsThrowable!>Deep<!> : Throwable()
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
