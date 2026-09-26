// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8, 12
// An import alias of a world-writeable project constant. Go reports the
// import line, where the name MODE_WORLD_WRITABLE appears, and so does FIR.
package test

import android.content.Context
import test.Other.<!WorldWriteableFiles!>MODE_WORLD_WRITABLE<!> as WW
import java.io.FileOutputStream

object Other {
    const val MODE_WORLD_WRITABLE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
}

class AliasedProject(private val context: Context) {
    // Deliberate improvement: Go misses both reads, because the alias does
    // not spell the constant's name; FIR resolves them to Other's constant,
    // whose value is Context.MODE_WORLD_WRITEABLE.
    fun open(): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>WW<!>)

    fun template(): String = "mode=$<!WorldWriteableFiles!>WW<!>"
}
