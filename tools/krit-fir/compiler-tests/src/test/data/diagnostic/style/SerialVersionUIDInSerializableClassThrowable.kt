// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 31, 44, 50, 53
// Every Throwable is Serializable through java.lang.Throwable, so the message
// is true of any exception without serialVersionUID. Like Go, an exception is
// reported only when Go's name-based evidence says it is Serializable: it or
// a base class names Serializable, or a supertype shares its simple name with
// a Serializable source class.
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

sealed class Resource : Serializable {
    companion object {
        private const val serialVersionUID = 1L
    }

    <!SerialVersionUIDInSerializableClass!>data<!> class Error(val message: String) : Resource()
}

// Go resolves `Error` by simple name to Resource.Error above, which is
// Serializable. The class really extends kotlin.Error, which is Serializable
// through Throwable, so the finding is true and FIR keeps it.
<!SerialVersionUIDInSerializableClass!>class<!> OtherFatal(msg: String) : Error(msg)

// The same, spelled with its package: Go takes the last name, `Error`.
<!SerialVersionUIDInSerializableClass!>class<!> FatalFailure(msg: String) : kotlin.Error(msg)
