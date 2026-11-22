// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go skips this FIR-resolved import call because a local Log declaration exists later in the file.
package test.logconditional.shadowing

import android.util.Log

private class FakeLog { fun d(tag: String, message: String) {} }
private fun compute(): String = "computed"

fun declarationOrder() {
    // Go treats any Log declaration anywhere in this file as a possible receiver shadow.
    <!LogConditional!>Log.d("Tag", compute())<!>
    val Log = FakeLog()
    Log.d("Tag", compute())
}
