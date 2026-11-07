// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 19, 21, 23, 25, 36, 41, 44, 59, 62, 69, 81, 83, 86, 91, 95, 101, 107, 118, 124, 130, 136, 140
// Serializable classes with no serialVersionUID, reported like Go on the
// class name.
package test

import java.io.Externalizable
import java.io.ObjectInput
import java.io.ObjectOutput
import java.io.Serializable

class <!SerialVersionUIDInSerializableClass!>Plain<!> : Serializable

// A qualified `java.io.Serializable` and `Externalizable` are not matched, like detekt.
class Qualified : java.io.Serializable {
    val name = "qualified"
}

data class <!SerialVersionUIDInSerializableClass!>Point<!>(val x: Int, val y: Int) : Serializable

abstract class <!SerialVersionUIDInSerializableClass!>AbstractBase<!> : Serializable

sealed class <!SerialVersionUIDInSerializableClass!>State<!> : Serializable

open class <!SerialVersionUIDInSerializableClass!>Generic<!><T>(val value: T) : Comparable<Generic<T>>, Serializable {
    override fun compareTo(other: Generic<T>): Int = 0
}

class Custom : Externalizable {
    override fun writeExternal(out: ObjectOutput) {}
    override fun readExternal(input: ObjectInput) {}
}

// The report lands on the class name, not on the annotation line above it.
@Suppress("unused")
class <!SerialVersionUIDInSerializableClass!>Annotated<!> : Serializable

/**
 * A KDoc above the declaration does not move the report off the class name.
 */
class <!SerialVersionUIDInSerializableClass!>Documented<!> : Serializable

@JvmInline
value class <!SerialVersionUIDInSerializableClass!>Wrapped<!>(val raw: Int) : Serializable

// Serializable only through a parent class or interface is not reported,
// like detekt, which reads only the declared supertype entries.
data class Loaded(val items: Int) : State()

class Derived : AbstractBase()

// An interface is never reported (see
// SerialVersionUIDInSerializableClassInterface.kt).
interface Marker : Serializable

class Marked : Marker

// A constructor property is instance state, not the class's serialVersionUID.
class <!SerialVersionUIDInSerializableClass!>CtorProperty<!>(val serialVersionUID: Long) : Serializable

// A companion object that declares other members does not count.
class <!SerialVersionUIDInSerializableClass!>OtherCompanion<!> : Serializable {
    companion object {
        const val VERSION = 1L
    }
}

// A property in a nested object or a function body is not the class's.
class <!SerialVersionUIDInSerializableClass!>NestedHolder<!> : Serializable {
    object Holder {
        const val serialVersionUID = 1L
    }

    fun version(): Long {
        val serialVersionUID = 1L
        return serialVersionUID
    }
}

class Outer {
    class <!SerialVersionUIDInSerializableClass!>Nested<!> : Serializable

    inner class <!SerialVersionUIDInSerializableClass!>Inner<!> : Serializable

    companion object {
        class <!SerialVersionUIDInSerializableClass!>InCompanion<!> : Serializable
    }
}

object Registry {
    class <!SerialVersionUIDInSerializableClass!>Entry<!> : Serializable
}

fun local(): Any {
    class <!SerialVersionUIDInSerializableClass!>Local<!> : Serializable
    return Local()
}

fun annotatedLocal(): Any {
    @Suppress("unused")
    class <!SerialVersionUIDInSerializableClass!>AnnotatedLocal<!> : Serializable
    return AnnotatedLocal()
}

// Local classes, one extending another, resolved without a class-id lookup.
fun localHierarchy(): Any {
    open class <!SerialVersionUIDInSerializableClass!>LocalBase<!> : Serializable {
        private val serialVersionUID = 1L
    }

    class LocalDerived : LocalBase()
    return LocalDerived()
}

// A member of an object expression.
val anonymousMember = object {
    fun make(): Any {
        class <!SerialVersionUIDInSerializableClass!>InAnonymous<!> : Serializable
        return InAnonymous()
    }
}

// Only a companion property counts, and it must be private const val Long.
class <!SerialVersionUIDInSerializableClass!>BodyProperty<!> : Serializable {
    val serialVersionUID = 4L
}

class WrongSignature : Serializable {
    companion object {
        <!SerialVersionUIDInSerializableClass!>val serialVersionUID<!> = 3L
    }
}

class NotConst : Serializable {
    companion object {
        <!SerialVersionUIDInSerializableClass!>private val serialVersionUID: Long<!> = 3L
    }
}

object <!SerialVersionUIDInSerializableClass!>Singleton<!> : Serializable

object CorrectSingleton : Serializable {
    private const val serialVersionUID: Long = 1L
}
