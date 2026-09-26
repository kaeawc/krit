// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 17
// Divergence (precision): a project constant named MODE_WORLD_READABLE, here
// holding MODE_PRIVATE, imported and passed as the file mode. Go reports the
// import and the use because it matches the identifier's text; neither names
// the platform's world-readable mode, and the file is opened private, so FIR
// reports neither.
package test

import android.content.Context
import test.LegacyModes.MODE_WORLD_READABLE

object LegacyModes {
    const val MODE_WORLD_READABLE = Context.MODE_PRIVATE
}

fun lookalike(context: Context) = context.getSharedPreferences("data", MODE_WORLD_READABLE)
