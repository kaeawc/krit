// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Pinned conservative Go recall differences: an unknown uppercase/imported reference may be an external constant,
// and a qualified Java property may be a field (a reference, not work). FIR resolves the parameter
// and Java getter, so it reports the work. Go does not guess and risk a false positive.
package test.logconditional.evidence

import android.util.Log
import java.util.Locale
import java.io.File
import java.io.File.separator

fun evidence(UPPERCASE: String, locale: Locale) {
    <!LogConditional!>Log.d("Tag", "x$UPPERCASE")<!>
    <!LogConditional!>Log.d("Tag", locale.displayName)<!>
    <!LogConditional!>Log.d("Tag", "x$separator")<!>
    Log.d("Tag", File.separator)
    Log.d("Tag", "x${Integer.MAX_VALUE}")
}
