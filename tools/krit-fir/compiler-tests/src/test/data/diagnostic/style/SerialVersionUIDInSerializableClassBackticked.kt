// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
// A backticked serialVersionUID is not found by name text, so Go and detekt report the class.
package test

import java.io.Serializable

class <!SerialVersionUIDInSerializableClass!>Backticked<!> : Serializable {
    companion object {
        private const val `serialVersionUID` = 1L
    }
}
