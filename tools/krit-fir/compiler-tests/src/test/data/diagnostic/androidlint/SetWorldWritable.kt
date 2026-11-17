// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 21, 25, 29, 33, 39, 43, 47, 50, 53, 57
// Positives for SetWorldWritable: `file.setWritable(true, false)` on a java.io.File makes the file writable by everyone. The
// permission and owner-only arguments may be literals, a negation, or a `val` of this file that holds one.
package test.setworldwritable

import java.io.File

const val GRANT = true
const val OWNER_ONLY = false

class Exporter(private val shared: File) {
    private val grant = true
    private val everyone = !true

    fun literals(file: File) {
        <!SetWorldWritable!>file.setWritable(true, false)<!>
    }

    fun field() {
        <!SetWorldWritable!>shared.setWritable(true, false)<!>
    }

    fun safeCall(file: File?) {
        <!SetWorldWritable!>file?.setWritable(true, false)<!>
    }

    fun constants(file: File) {
        <!SetWorldWritable!>file.setWritable(GRANT, OWNER_ONLY)<!>
    }

    fun memberValues(file: File) {
        <!SetWorldWritable!>file.setWritable(grant, everyone)<!>
    }

    fun localValues(file: File) {
        val permit = true
        val ownerOnly = false
        <!SetWorldWritable!>file.setWritable(permit, ownerOnly)<!>
    }

    fun negated(file: File) {
        <!SetWorldWritable!>file.setWritable(!false, !true)<!>
    }

    fun parenthesized(file: File) {
        <!SetWorldWritable!>file.setWritable((true), (false))<!>
    }

    fun chained(): Boolean = <!SetWorldWritable!>File("/tmp/x").setWritable(true, false)<!>

    fun inLambda(file: File) {
        run { <!SetWorldWritable!>file.setWritable(true, false)<!> }
    }

    fun multiLine(file: File) {
        <!SetWorldWritable!>file<!>
            .setWritable(
                true,
                false,
            )
    }
}
