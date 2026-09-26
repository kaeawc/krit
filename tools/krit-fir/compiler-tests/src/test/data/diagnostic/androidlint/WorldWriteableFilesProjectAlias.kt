// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 14, 21, 26, 28, 30, 34, 36, 39, 41, 44, 45
// Positive: project properties named MODE_WORLD_WRITEABLE or
// MODE_WORLD_WRITABLE whose declared value is the Android constant (directly,
// through another property, a getter, or a lazy delegate) or an Int literal
// with the world-writeable bit set. Go reports every read and the import by
// name; FIR reports them because the value read is a world-writeable mode.
package test

import android.content.Context
<!WorldWriteableFiles!>import test.MODE_WORLD_WRITABLE<!>
import java.io.FileOutputStream

const val MODE_WORLD_WRITABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

object LegacyModes {
    const val MODE_WORLD_WRITEABLE = 2
}

class Chained(private val context: Context) {
    private val shared = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

    val MODE_WORLD_WRITEABLE = shared

    val MODE_WORLD_WRITABLE: Int
        get() = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

    fun viaChain(): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun viaGetter(): FileOutputStream = context.openFileOutput("b.txt", this.<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)
}

class LazyModes(private val context: Context) {
    private val MODE_WORLD_WRITABLE by lazy { Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> }

    fun open(): FileOutputStream = context.openFileOutput("c.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)
}

fun topLevel(context: Context): FileOutputStream = context.openFileOutput("d.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)

fun literal(context: Context): FileOutputStream = context.openFileOutput("e.txt", LegacyModes.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

fun local(context: Context): FileOutputStream {
    val MODE_WORLD_WRITABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> or Context.MODE_WORLD_READABLE
    return context.openFileOutput("f.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)
}
