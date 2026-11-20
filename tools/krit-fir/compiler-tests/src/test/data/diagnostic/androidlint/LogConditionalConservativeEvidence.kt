// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Pinned conservative Go recall differences: an unresolved reference may be an external constant,
// and a qualified Java property may be a field (a reference, not work). FIR resolves the parameter
// and Java getter, so it reports the work. Go resolves in-scope parameters but does not guess at imports.
package test.logconditional.evidence

import android.util.Log
import java.util.Locale
import java.io.File
import java.io.File.separator
import test.logconditional.external.Config
import test.logconditional.external.Config.runtimePrefix

fun evidence(UPPERCASE: String, locale: Locale) {
    <!LogConditional!>Log.d("Tag", "x$UPPERCASE")<!>
    <!LogConditional!>Log.d("Tag", locale.displayName)<!>
    <!LogConditional!>Log.d("Tag", "x$separator")<!>
    Log.d("Tag", File.separator)
    Log.d("Tag", "x${Integer.MAX_VALUE}")
}

fun ready() { val msg = "ready"; Log.d("Tag", msg + "!") }
// Go still folds a same-named literal val in another scope; FIR/Lint bind the parameter and report.
fun failed(msg: String) { <!LogConditional!>Log.d("Tag", msg + "!")<!> }

// Go cannot prove these other-file Kotlin members are non-constant; FIR/Lint resolve them and report.
fun externalValues() {
    <!LogConditional!>Log.d("Tag", "x${Config.runtimePrefix}")<!>
    <!LogConditional!>Log.d("Tag", "x$runtimePrefix")<!>
    <!LogConditional!>Log.d("Tag", "x" + Config.runtimePrefix)<!>
    <!LogConditional!>Log.d("Tag", "x" + runtimePrefix)<!>
}
