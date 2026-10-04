// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15x2, 17x2, 19x2, 21x2, 23x2, 24, 28x2, 29x2, 30, 33x2, 37x2, 38, 42, 43, 50, 54, 55, 60, 61
// Positive: parameters named MODE_WORLD_WRITEABLE or MODE_WORLD_WRITABLE
// whose default is world-writeable. A caller that omits the argument opens a
// world-writeable file, so FIR reports the parameter's name and every read of
// it, as Go does by name. A named argument whose value may be world-writeable
// is reported on its label too, as Go reports that identifier. A parameter of
// an override inherits its default, and a lambda's parameter has no visible
// value, so their reads are reported as well.
package test

import android.content.Context
import java.io.FileOutputStream

annotation class Mode(val <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

class Modes(val <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

data class DataModes(val <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

fun copied(modes: DataModes): DataModes = modes.copy(<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> = modes.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

fun defaulted(context: Context, <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>): FileOutputStream =
    context.openFileOutput("a.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

fun callers(context: Context, modes: Modes): List<Any> = listOf(
    defaulted(context),
    defaulted(context, <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>),
    Modes(<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!> = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>),
    modes.<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>,
)

@Mode(<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
fun annotated() {}

open class Opener {
    open fun open(context: Context, <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>): FileOutputStream =
        context.openFileOutput("b.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}

class SubOpener : Opener() {
    override fun open(context: Context, <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>: Int): FileOutputStream =
        context.openFileOutput("c.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}

// A lambda's parameter is supplied by whatever invokes the lambda, so its
// value is unknown: its read is reported, as Go reports it. Go skips the
// parameter's name (a variable declaration to tree-sitter), and so does FIR.
fun lambda(context: Context, modes: List<Int>): List<FileOutputStream> =
    modes.map { MODE_WORLD_WRITEABLE -> context.openFileOutput("e.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>) }

// An anonymous function's parameter is supplied by whoever calls it, so its
// name and its read are reported, as Go reports them.
val anonymous = fun(context: Context, <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>: Int): FileOutputStream =
    context.openFileOutput("f.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)

// Precision: Go reports the parameter and its read by name, but the default
// is MODE_PRIVATE and any other value comes from the caller, whose own read
// of a world-writeable mode is reported where it is written.
fun privateDefault(context: Context, MODE_WORLD_WRITABLE: Int = Context.MODE_PRIVATE): FileOutputStream =
    context.openFileOutput("d.txt", MODE_WORLD_WRITABLE)
