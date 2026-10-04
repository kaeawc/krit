// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for WorldReadableFiles, reported by neither Go nor FIR: the name in
// a comment, a string, and a KDoc link; the private and world-writeable modes
// (WorldWriteableFiles covers the latter); and a property whose name only
// starts with MODE_WORLD_READABLE.
package test

import android.content.Context

class PrefsHelper(private val context: Context) {
    // MODE_WORLD_READABLE is deprecated; do not use it.
    private val warning = "Do not use MODE_WORLD_READABLE in your app"

    /** Never pass [Context.MODE_WORLD_READABLE]. */
    fun prefs() = context.getSharedPreferences("data", Context.MODE_PRIVATE)

    fun writeable() = context.openFileOutput("f", Context.MODE_WORLD_WRITEABLE)

    val MODE_WORLD_READABLE_LABEL = "MODE_WORLD_READABLE"
}
