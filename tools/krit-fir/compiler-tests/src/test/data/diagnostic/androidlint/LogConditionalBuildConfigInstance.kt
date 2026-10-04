// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 23, 30
// A DEBUG member of a class named BuildConfig read through an instance, an
// implicit `with` receiver, or `this` inside the class is not the app's
// static BuildConfig.DEBUG flag. Go matches only the condition text
// `BuildConfig.DEBUG`, so it reports each call below, and FIR matches.
package test.logconditional.instance

import android.util.Log

class BuildConfig {
    val DEBUG: Boolean = true

    fun member() {
        if (DEBUG) {
            <!LogConditional!>Log.d("Tag", "DEBUG through this")<!>
        }
    }
}

fun instanceReceiver(cfg: BuildConfig) {
    if (cfg.DEBUG) {
        <!LogConditional!>Log.d("Tag", "DEBUG through an instance")<!>
    }
}

fun implicitReceiver(cfg: BuildConfig) {
    with(cfg) {
        if (DEBUG) {
            <!LogConditional!>Log.d("Tag", "DEBUG through an implicit receiver")<!>
        }
    }
}
