// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Pinned recall difference: FIR resolves a star import; Go requires an explicit import for Log.
package test.logconditional.starimport

import android.util.*

fun starImport() {
    <!LogConditional!>Log.i("Tag", compute())<!>
}

fun guarded() {
    if (Log.isLoggable("Tag", Log.INFO)) {
        Log.i("Tag", compute())
    }
}

private fun compute(): String = "computed"
