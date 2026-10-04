// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12
// Divergence (precision): MODE_PRIVATE imported under the alias
// MODE_WORLD_READABLE. Go does not report the import directive, but it reports
// the alias's use by the identifier's text; the use is the private mode and
// the file is opened private, so FIR does not report it.
package test

import android.content.Context
import android.content.Context.MODE_PRIVATE as MODE_WORLD_READABLE

fun aliasedPrivate(context: Context) = context.getSharedPreferences("data", MODE_WORLD_READABLE)
