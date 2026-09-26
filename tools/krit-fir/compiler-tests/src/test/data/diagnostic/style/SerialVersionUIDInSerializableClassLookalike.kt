// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 18, 21
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

// A qualified reference still reaches the real interface, like Go.
<!SerialVersionUIDInSerializableClass!>class<!> RealMarker : java.io.Serializable
