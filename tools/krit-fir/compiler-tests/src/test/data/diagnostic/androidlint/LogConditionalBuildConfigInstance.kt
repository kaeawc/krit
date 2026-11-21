// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Any non-literal-true if condition guards message work in both engines.
package test.logconditional.instance

import android.util.Log

class BuildConfig {
    val DEBUG: Boolean = true

    fun member() {
        if (DEBUG) {
            Log.d("Tag", compute())
        }
    }
}

fun instanceReceiver(cfg: BuildConfig) {
    if (cfg.DEBUG) {
        Log.d("Tag", compute())
    }
}

fun implicitReceiver(cfg: BuildConfig) {
    with(cfg) {
        if (DEBUG) {
            Log.d("Tag", compute())
        }
    }
}

private fun compute(): String = "computed"
