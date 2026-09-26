// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 30, 36, 42
// Divergence (precision): each call below is wrapped in Log.isLoggable() or
// BuildConfig.DEBUG, so the message ("Unconditional logging call. Wrap in
// Log.isLoggable() or BuildConfig.DEBUG") is false of it and FIR does not
// report it. Go recognizes a guard by its spelling only: `Log.isLoggable`
// with the receiver spelled `Log`, and the condition text `BuildConfig.DEBUG`,
// so it reports these calls.
package test.logconditional.divergence

import android.util.Log
import android.util.Log.isLoggable
import android.util.Log as AndroidLog
import test.logconditional.divergence.BuildConfig.DEBUG
import test.logconditional.divergence.BuildConfig as Config

// The app's generated BuildConfig; the golden declares its own.
object BuildConfig {
    val DEBUG: Boolean = true
}

fun staticallyImportedIsLoggable() {
    if (isLoggable("Tag", Log.DEBUG)) {
        Log.d("Tag", "guarded by a statically imported isLoggable")
    }
}

fun aliasedIsLoggable() {
    if (AndroidLog.isLoggable("Tag", Log.DEBUG)) {
        Log.d("Tag", "guarded by an aliased isLoggable")
    }
}

fun staticallyImportedDebug() {
    if (DEBUG) {
        Log.d("Tag", "guarded by a statically imported BuildConfig.DEBUG")
    }
}

fun aliasedBuildConfig() {
    if (Config.DEBUG) {
        Log.d("Tag", "guarded by an aliased BuildConfig")
    }
}
