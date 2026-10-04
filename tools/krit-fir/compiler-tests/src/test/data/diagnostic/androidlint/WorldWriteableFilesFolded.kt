// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26x2, 31, 33
// Constant-folded values. A project property named MODE_WORLD_WRITEABLE or
// MODE_WORLD_WRITABLE whose initializer is an Int expression (`1 shl 1`,
// `0x0002 or 0x0001`) is evaluated: when the value has the world-writeable
// bit (0x2) set, every read is reported, as Go reports it by name.
package test

import android.content.Context
import java.io.FileOutputStream

object Folded {
    const val MODE_WORLD_WRITEABLE = 1 shl 1
    const val MODE_WORLD_WRITABLE = 0x0002 or 0x0001
}

object Shifted {
    const val MODE_WORLD_WRITEABLE = 1 shl 2
}

object Combined {
    val MODE_WORLD_WRITABLE = Context.MODE_PRIVATE or Context.MODE_WORLD_READABLE
}

fun folded(context: Context): FileOutputStream =
    context.openFileOutput("a.txt", Folded.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> or Folded.<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!>)

// Precision: Go reports these reads by name, but the values are 4
// (`1 shl 2`) and 1 (MODE_PRIVATE or MODE_WORLD_READABLE); neither has the
// world-writeable bit, so the file is not opened world-writeable.
fun shifted(context: Context): FileOutputStream = context.openFileOutput("b.txt", Shifted.MODE_WORLD_WRITEABLE)

fun combined(context: Context): FileOutputStream = context.openFileOutput("c.txt", Combined.MODE_WORLD_WRITABLE)
