// RENDER_DIAGNOSTICS_FULL_TEXT
// Spellings of the ObjectInputStream constructor that Go's name match misses.
package test

import java.io.InputStream
import java.io.ObjectInputStream as Ois

typealias SerialInput = java.io.ObjectInputStream

class Decoder {
    // Go misses these because it matches the spelled callee name
    // `ObjectInputStream`; FIR is correct because each call constructs a
    // java.io.ObjectInputStream (import alias, type alias, backticked
    // qualified name).
    fun importAlias(input: InputStream): Ois = <!JavaObjectInputStream!>Ois(input)<!>

    fun typeAlias(input: InputStream): SerialInput = <!JavaObjectInputStream!>SerialInput(input)<!>

    fun backticked(input: InputStream): Ois = <!JavaObjectInputStream!>java.io.`ObjectInputStream`(input)<!>
}
