// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 17, 21, 25, 29, 33, 37, 43, 45
// Positive: a world-writeable project property named MODE_WORLD_WRITEABLE or
// MODE_WORLD_WRITABLE, inherited from a generic supertype. The read resolves
// to a substitution override, which has no initializer or getter of its own;
// FIR unwraps it to the declaration, whose value is world-writeable. Go
// reports every read by name, and so does FIR. The non-generic shapes and a
// member of an object inherited from a generic class are reported the same
// way.
package test

import android.content.Context
<!WorldWriteableFiles!>import test.SharedModes.MODE_WORLD_WRITEABLE<!>
import java.io.FileOutputStream

open class Base<T> {
    val MODE_WORLD_WRITEABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
}

class Sub(private val context: Context) : Base<String>() {
    fun open(): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}

interface Getter<T> {
    val MODE_WORLD_WRITABLE: Int get() = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
}

class GetterImpl(private val context: Context) : Getter<String> {
    fun open(): FileOutputStream = context.openFileOutput("b.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)
}

open class PlainBase {
    val MODE_WORLD_WRITEABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
}

class PlainSub(private val context: Context) : PlainBase() {
    fun open(): FileOutputStream = context.openFileOutput("c.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}

object SharedModes : Base<Int>()

fun viaObject(context: Context): FileOutputStream =
    context.openFileOutput("d.txt", SharedModes.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

fun viaImport(context: Context): FileOutputStream = context.openFileOutput("e.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
