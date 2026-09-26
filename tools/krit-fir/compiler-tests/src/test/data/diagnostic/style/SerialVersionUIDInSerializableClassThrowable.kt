// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 29
// Every Throwable is Serializable through java.lang.Throwable, but, like Go,
// an exception is reported only when it or a base class declares
// Serializable itself.
package test

import java.io.IOException
import java.io.Serializable

class AppException(message: String) : RuntimeException(message)

class Disconnected : IOException("disconnected")

object NotReady : IllegalStateException("not ready")

open class AppFailure : Exception()

class Timeout : AppFailure()

<!SerialVersionUIDInSerializableClass!>class<!> Explicit : Exception(), Serializable

open class SerializableFailure : RuntimeException(), Serializable {
    companion object {
        private const val serialVersionUID = 1L
    }
}

<!SerialVersionUIDInSerializableClass!>class<!> DerivedFailure : SerializableFailure()

class Declared : Exception(), Serializable {
    companion object {
        private const val serialVersionUID = 1L
    }
}
