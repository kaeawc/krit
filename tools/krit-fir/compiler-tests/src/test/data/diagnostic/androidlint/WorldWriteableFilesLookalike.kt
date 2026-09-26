// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 21, 23, 25, 27, 28, 31, 32, 33, 34, 35, 36, 37
// Local lookalikes: project declarations that only share the constant's name.
// Go reports each identifier spelled MODE_WORLD_WRITEABLE or
// MODE_WORLD_WRITABLE (only a property's declared name is skipped): the
// import, reads of a property whose value is not world-writeable, a parameter
// and its reads, a named argument, a constructor property, an enum entry, and
// a function and its call. FIR reports none of them, because none is Android's
// Context.MODE_WORLD_WRITEABLE and none reads a world-writeable value.
package test

import android.content.Context
import test.Flags.MODE_WORLD_WRITEABLE
import java.io.FileOutputStream

object Flags {
    const val MODE_WORLD_WRITEABLE = 0
    const val MODE_WORLD_WRITABLE = Context.MODE_PRIVATE
}

enum class Access { MODE_WORLD_WRITEABLE, PRIVATE }

class Settings(val MODE_WORLD_WRITABLE: Int)

fun MODE_WORLD_WRITABLE(): Int = 0

fun open(context: Context, MODE_WORLD_WRITEABLE: Int): FileOutputStream =
    context.openFileOutput("a.txt", MODE_WORLD_WRITEABLE)

fun uses(context: Context, settings: Settings): List<Any> = listOf(
    Flags.MODE_WORLD_WRITEABLE,
    Flags.MODE_WORLD_WRITABLE,
    MODE_WORLD_WRITEABLE,
    Access.MODE_WORLD_WRITEABLE,
    settings.MODE_WORLD_WRITABLE,
    MODE_WORLD_WRITABLE(),
    open(context, MODE_WORLD_WRITEABLE = 0),
)
