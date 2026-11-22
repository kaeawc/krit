// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Pinned recall differences: FIR resolves import/type aliases and static imports; Go uses receiver/import evidence.
package test.logconditional.recall

import android.util.Log as AndroidLog
import android.util.Log.d

typealias Logger = android.util.Log

fun qualified() {
    <!LogConditional!>android.util.Log.d("Tag", compute())<!>
}

fun importAlias() {
    <!LogConditional!>AndroidLog.d("Tag", compute())<!>
}

fun typeAlias() {
    <!LogConditional!>Logger.i("Tag", compute())<!>
}

fun staticImport() {
    <!LogConditional!>d("Tag", compute())<!>
}

fun guardedSpellings() {
    if (AndroidLog.isLoggable("Tag", AndroidLog.DEBUG)) {
        AndroidLog.d("Tag", compute())
        d("Tag", compute())
    }
}

private fun compute(): String = "computed"
