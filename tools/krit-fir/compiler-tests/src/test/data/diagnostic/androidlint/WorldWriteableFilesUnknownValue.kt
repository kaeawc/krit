// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20x2, 23, 26, 27, 28, 29, 30, 36, 40, 45, 53, 57, 64, 70x2, 73, 77
// Positive: project properties named MODE_WORLD_WRITEABLE or
// MODE_WORLD_WRITABLE whose value the declaration does not pin down: a var
// (reassigned elsewhere), a val assigned in an init block, a function-call
// initializer, an abstract or open property (an override supplies the
// value), and a loop variable. FIR reports a read unless the value is
// provably not world-writeable, so each read Go reports by name is kept.
// An assignment of a value that may be world-writeable is reported on the
// assigned name too, as Go reports that identifier.
package test

import android.content.Context
import java.io.FileOutputStream

class Reassigned(private val context: Context) {
    private var MODE_WORLD_WRITEABLE = 0

    init {
        <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
    }

    fun open(): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun bump() {
        <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>++
        <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> += 2
        ++<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
        <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>--
        this.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> -= 1
    }

    // Precision: Go reports the assigned name, but the value written is 0,
    // not a world-writeable mode.
    fun reset() {
        MODE_WORLD_WRITEABLE = 0
    }
}

fun mode(): Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

class FromCall(private val context: Context) {
    val MODE_WORLD_WRITEABLE = mode()

    fun open(): FileOutputStream = context.openFileOutput("b.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}

interface Modes {
    val MODE_WORLD_WRITEABLE: Int
}

object ModesImpl : Modes {
    override val MODE_WORLD_WRITEABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
}

fun abstractRead(context: Context, modes: Modes): FileOutputStream =
    context.openFileOutput("c.txt", modes.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

open class OpenModes {
    open val MODE_WORLD_WRITABLE = 0
}

fun openRead(context: Context, modes: OpenModes): FileOutputStream =
    context.openFileOutput("d.txt", modes.<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)

class LateModes(private val context: Context) {
    val MODE_WORLD_WRITABLE: Int

    init {
        <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!> = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
    }

    fun open(): FileOutputStream = context.openFileOutput("e.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)
}

fun loop(context: Context, modes: List<Int>) {
    for (MODE_WORLD_WRITEABLE in modes) context.openFileOutput("f.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}
