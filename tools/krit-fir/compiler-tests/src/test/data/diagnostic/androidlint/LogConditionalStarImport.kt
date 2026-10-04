// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): with a star import, `Log` is android.util.Log and the
// call is unguarded, so FIR reports it. Go misses it: it needs an explicit
// `import android.util.Log` to treat the receiver `Log` as Android's.
package test.logconditional.starimport

import android.util.*

fun starImport() {
    <!LogConditional!>Log.i("Tag", "m")<!>
}

fun guarded() {
    if (Log.isLoggable("Tag", Log.INFO)) {
        Log.i("Tag", "guarded")
    }
}
