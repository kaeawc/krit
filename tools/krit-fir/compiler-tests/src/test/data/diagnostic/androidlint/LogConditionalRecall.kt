// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each call below is an unguarded android.util.Log level
// call, and FIR reports it. Go misses them: it needs the receiver spelled
// `Log` and an explicit `import android.util.Log`, so a fully qualified call
// without that import, an import alias, a typealias, and a statically
// imported level function are other spellings (a star import is in
// LogConditionalStarImport).
package test.logconditional.recall

import android.util.Log as AndroidLog
import android.util.Log.d

typealias Logger = android.util.Log

fun qualified() {
    <!LogConditional!>android.util.Log.d("Tag", "m")<!>
}

fun importAlias() {
    <!LogConditional!>AndroidLog.e("Tag", "m")<!>
}

fun typeAlias() {
    <!LogConditional!>Logger.w("Tag", "m")<!>
}

fun staticImport() {
    <!LogConditional!>d("Tag", "m")<!>
}

fun guardedSpellings() {
    if (AndroidLog.isLoggable("Tag", AndroidLog.DEBUG)) {
        AndroidLog.d("Tag", "guarded")
        d("Tag", "guarded")
    }
}
