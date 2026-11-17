// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 21, 25, 29, 33, 39, 43, 47, 50, 53, 57
// Positives for SetWorldReadable: `file.setReadable(true, false)` on a java.io.File makes the file readable by everyone. The
// permission and owner-only arguments may be literals, a negation, or a `val` of this file that holds one.
package test.setworldreadable

import java.io.File

const val GRANT = true
const val OWNER_ONLY = false

class Exporter(private val shared: File) {
    private val grant = true
    private val everyone = !true

    fun literals(file: File) {
        <!SetWorldReadable!>file.setReadable(true, false)<!>
    }

    fun field() {
        <!SetWorldReadable!>shared.setReadable(true, false)<!>
    }

    fun safeCall(file: File?) {
        <!SetWorldReadable!>file?.setReadable(true, false)<!>
    }

    fun constants(file: File) {
        <!SetWorldReadable!>file.setReadable(GRANT, OWNER_ONLY)<!>
    }

    fun memberValues(file: File) {
        <!SetWorldReadable!>file.setReadable(grant, everyone)<!>
    }

    fun localValues(file: File) {
        val permit = true
        val ownerOnly = false
        <!SetWorldReadable!>file.setReadable(permit, ownerOnly)<!>
    }

    fun negated(file: File) {
        <!SetWorldReadable!>file.setReadable(!false, !true)<!>
    }

    fun parenthesized(file: File) {
        <!SetWorldReadable!>file.setReadable((true), (false))<!>
    }

    fun chained(): Boolean = <!SetWorldReadable!>File("/tmp/x").setReadable(true, false)<!>

    fun inLambda(file: File) {
        run { <!SetWorldReadable!>file.setReadable(true, false)<!> }
    }

    fun multiLine(file: File) {
        <!SetWorldReadable!>file<!>
            .setReadable(
                true,
                false,
            )
    }
}
