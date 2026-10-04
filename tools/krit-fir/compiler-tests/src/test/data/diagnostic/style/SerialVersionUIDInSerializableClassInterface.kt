// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 16, 25
// Interfaces are not serialized; Java serialization ignores a
// serialVersionUID declared on an interface, so an interface is never missing
// one.
package test

import java.io.Serializable

// Go reports this because it visits every class_declaration, interfaces
// included; FIR is correct to drop it because an interface is not a
// Serializable class.
interface Payload : Serializable

// Go reports this for the same reason; FIR drops it.
sealed interface Event : Serializable

// Neither Go nor FIR reports this: FIR drops interfaces, and Go does not
// see a fun interface as a class_declaration.
fun interface Callback : Serializable {
    fun invoke()
}

// The class implementing the interface is still reported, like Go.
<!SerialVersionUIDInSerializableClass!>class<!> Click : Event
