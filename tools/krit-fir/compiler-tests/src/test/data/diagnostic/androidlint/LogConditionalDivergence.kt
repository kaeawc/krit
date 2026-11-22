// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Any non-literal-true if condition guards message work in both engines.
package test.logconditional.divergence

import android.util.Log
import android.util.Log.isLoggable
import android.util.Log as AndroidLog
import test.logconditional.divergence.BuildConfig.DEBUG
import test.logconditional.divergence.BuildConfig as Config

object BuildConfig {
    val DEBUG: Boolean = true
}

fun staticallyImportedIsLoggable() {
    if (isLoggable("Tag", Log.DEBUG)) {
        Log.d("Tag", compute())
    }
}

fun aliasedIsLoggable() {
    if (AndroidLog.isLoggable("Tag", Log.DEBUG)) {
        Log.d("Tag", compute())
    }
}

fun staticallyImportedDebug() {
    if (DEBUG) {
        Log.d("Tag", compute())
    }
}

fun aliasedBuildConfig() {
    if (Config.DEBUG) {
        Log.d("Tag", compute())
    }
}

typealias GeneratedConfig = test.logconditional.divergence.BuildConfig

fun typeAliasedBuildConfig() {
    if (GeneratedConfig.DEBUG) {
        Log.d("Tag", compute())
    }
}

private fun compute(): String = "computed"
