// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 18, 23, 25, 28, 33, 35, 37, 41
// Local lookalikes: interfaces in this package named Serializable and
// Externalizable are not java.io types, so these classes are not
// Serializable.
package test

interface Serializable

interface Externalizable

// Go reports this because the supertype is named Serializable; FIR is correct
// to drop it because test.Serializable is not java.io.Serializable.
class LocalMarker : Serializable

// Go reports this because the supertype is named Externalizable; FIR is
// correct to drop it because test.Externalizable is not java.io.Serializable.
class LocalExternal : Externalizable

// Go reports both because it follows the base class by name to a supertype
// named Serializable; FIR is correct to drop them because neither class is
// java.io.Serializable.
open class LookalikeBase : Serializable

class LookalikeDerived : LookalikeBase()

// A qualified reference still reaches the real interface, like Go.
<!SerialVersionUIDInSerializableClass!>class<!> RealMarker : java.io.Serializable

// Go reports these by the lookalike's name, and here the finding is true: a
// Throwable is java.io.Serializable through java.lang.Throwable, whatever
// the lookalike is.
<!SerialVersionUIDInSerializableClass!>class<!> LookalikeFailure : Exception(), Serializable

<!SerialVersionUIDInSerializableClass!>class<!> LookalikeExternalFailure : Exception(), Externalizable

<!SerialVersionUIDInSerializableClass!>open<!> class LookalikeBaseFailure : RuntimeException(), Serializable

// Go follows the base class by name to the lookalike; the class is a
// Throwable, so the finding is true.
<!SerialVersionUIDInSerializableClass!>class<!> LookalikeDerivedFailure : LookalikeBaseFailure()
