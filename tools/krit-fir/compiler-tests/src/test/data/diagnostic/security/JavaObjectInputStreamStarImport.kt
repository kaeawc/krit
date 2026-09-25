// RENDER_DIAGNOSTICS_FULL_TEXT
// A java.io star import satisfies Go's file gate, so the bare call is reported
// by both.
package test

import java.io.*

class Decoder {
    fun open(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}
