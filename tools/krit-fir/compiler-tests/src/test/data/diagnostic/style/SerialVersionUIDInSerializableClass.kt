// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 14, 18, 20, 22, 24, 28, 34, 40, 42, 47, 49, 53, 55, 58, 61, 68, 80, 82, 85, 90, 94, 99, 110, 117
// Serializable classes with no serialVersionUID, reported like Go on the
// declaration's first line.
package test

import java.io.Externalizable
import java.io.ObjectInput
import java.io.ObjectOutput
import java.io.Serializable

<!SerialVersionUIDInSerializableClass!>class<!> Plain : Serializable

<!SerialVersionUIDInSerializableClass!>class<!> Qualified : java.io.Serializable {
    val name = "qualified"
}

<!SerialVersionUIDInSerializableClass!>data<!> class Point(val x: Int, val y: Int) : Serializable

<!SerialVersionUIDInSerializableClass!>abstract<!> class AbstractBase : Serializable

<!SerialVersionUIDInSerializableClass!>sealed<!> class State : Serializable

<!SerialVersionUIDInSerializableClass!>open<!> class Generic<T>(val value: T) : Comparable<Generic<T>>, Serializable {
    override fun compareTo(other: Generic<T>): Int = 0
}

<!SerialVersionUIDInSerializableClass!>class<!> Custom : Externalizable {
    override fun writeExternal(out: ObjectOutput) {}
    override fun readExternal(input: ObjectInput) {}
}

// The report lands on the annotation, the first line of the declaration.
<!SerialVersionUIDInSerializableClass!>@Suppress("unused")<!>
class Annotated : Serializable

/**
 * A KDoc is not part of the declaration's first line.
 */
<!SerialVersionUIDInSerializableClass!>class<!> Documented : Serializable

<!SerialVersionUIDInSerializableClass!>@JvmInline<!>
value class Wrapped(val raw: Int) : Serializable

// Serializable through a source base class or interface, as Go's resolver
// follows it.
<!SerialVersionUIDInSerializableClass!>data<!> class Loaded(val items: Int) : State()

<!SerialVersionUIDInSerializableClass!>class<!> Derived : AbstractBase()

// Go reports this interface too; FIR drops it (see
// SerialVersionUIDInSerializableClassInterface.kt).
interface Marker : Serializable

<!SerialVersionUIDInSerializableClass!>class<!> Marked : Marker

// A constructor property is instance state, not the class's serialVersionUID.
<!SerialVersionUIDInSerializableClass!>class<!> CtorProperty(val serialVersionUID: Long) : Serializable

// A companion object that declares other members does not count.
<!SerialVersionUIDInSerializableClass!>class<!> OtherCompanion : Serializable {
    companion object {
        const val VERSION = 1L
    }
}

// A property in a nested object or a function body is not the class's.
<!SerialVersionUIDInSerializableClass!>class<!> NestedHolder : Serializable {
    object Holder {
        const val serialVersionUID = 1L
    }

    fun version(): Long {
        val serialVersionUID = 1L
        return serialVersionUID
    }
}

class Outer {
    <!SerialVersionUIDInSerializableClass!>class<!> Nested : Serializable

    <!SerialVersionUIDInSerializableClass!>inner<!> class Inner : Serializable

    companion object {
        <!SerialVersionUIDInSerializableClass!>class<!> InCompanion : Serializable
    }
}

object Registry {
    <!SerialVersionUIDInSerializableClass!>class<!> Entry : Serializable
}

fun local(): Any {
    <!SerialVersionUIDInSerializableClass!>class<!> Local : Serializable
    return Local()
}

fun annotatedLocal(): Any {
    <!SerialVersionUIDInSerializableClass!>@Suppress("unused")<!>
    class AnnotatedLocal : Serializable
    return AnnotatedLocal()
}

// Local classes, one extending another, resolved without a class-id lookup.
fun localHierarchy(): Any {
    open class LocalBase : Serializable {
        private val serialVersionUID = 1L
    }

    <!SerialVersionUIDInSerializableClass!>class<!> LocalDerived : LocalBase()
    return LocalDerived()
}

// A member of an object expression.
val anonymousMember = object {
    fun make(): Any {
        <!SerialVersionUIDInSerializableClass!>class<!> InAnonymous : Serializable
        return InAnonymous()
    }
}
