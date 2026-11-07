// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Interfaces are not serialized; Java serialization ignores a
// serialVersionUID declared on an interface, so an interface is never missing
// one.
package test

import java.io.Serializable

// Neither Go nor FIR reports an interface: it is not a Serializable class.
interface Payload : Serializable

sealed interface Event : Serializable

// Neither Go nor FIR reports a fun interface either.
fun interface Callback : Serializable {
    fun invoke()
}

// A class that only reaches the interface through another one is not reported, like detekt.
class Click : Event
