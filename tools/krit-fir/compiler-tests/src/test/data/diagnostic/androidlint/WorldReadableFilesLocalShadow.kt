// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15
// Positive and divergence (precision): the platform constant is imported, so
// both Go and FIR report the import. A local val then shadows it with
// MODE_PRIVATE; Go reports the local's use by the identifier's text, but the
// use is the private mode and the file is opened private, so FIR does not
// report it.
package test

import android.content.Context
import android.content.Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>

fun shadowed(context: Context) {
    val MODE_WORLD_READABLE = Context.MODE_PRIVATE
    context.getSharedPreferences("data", MODE_WORLD_READABLE)
}
