// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses. Go matches a direct supertype named Serializable
// or Externalizable and follows other supertypes only by simple name through
// source classes. Each of these is java.io.Serializable and declares no
// serialVersionUID.
package test

import java.io.Externalizable
import java.io.ObjectInput
import java.io.ObjectOutput
import java.io.Serializable as JavaSerializable
import java.util.Date

// Go misses this because ArrayList is Serializable only in the library.
<!SerialVersionUIDInSerializableClass!>class<!> Names : ArrayList<String>()

// Go misses this because Date is Serializable only in the library.
<!SerialVersionUIDInSerializableClass!>class<!> Timestamp : Date()

typealias Wire = java.io.Serializable

// Go misses this because it does not expand the type alias.
<!SerialVersionUIDInSerializableClass!>class<!> Aliased : Wire

// Go misses this because it does not follow the import alias.
<!SerialVersionUIDInSerializableClass!>class<!> ImportAliased : JavaSerializable

// Go misses this because it skips a delegated supertype.
<!SerialVersionUIDInSerializableClass!>class<!> Delegating(impl: JavaSerializable) : JavaSerializable by impl

abstract class ExternalBase : Externalizable {
    companion object {
        private const val serialVersionUID = 1L
    }

    override fun writeExternal(out: ObjectOutput) {}
    override fun readExternal(input: ObjectInput) {}
}

// Go misses this because its resolver matches only Serializable in a base
// class's supertypes, not Externalizable.
<!SerialVersionUIDInSerializableClass!>class<!> ExternalDerived : ExternalBase()

// Go misses these because it never visits an object declaration.
<!SerialVersionUIDInSerializableClass!>object<!> Singleton : JavaSerializable

sealed class Screen : JavaSerializable {
    companion object {
        private const val serialVersionUID = 1L
    }

    <!SerialVersionUIDInSerializableClass!>data<!> object Home : Screen()

    object Settings : Screen() {
        private const val serialVersionUID = 2L
    }
}
