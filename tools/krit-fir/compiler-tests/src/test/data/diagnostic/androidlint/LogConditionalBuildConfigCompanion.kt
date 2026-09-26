// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A condition written `BuildConfig.DEBUG` guards the call wherever DEBUG is
// declared: in the companion object of a class named BuildConfig, or in a
// supertype of an object named BuildConfig. Go matches the condition text
// `BuildConfig.DEBUG`, and FIR matches.
package test.logconditional.companion

import android.util.Log

class BuildConfig {
    companion object {
        const val DEBUG: Boolean = true
    }
}

fun companionGuard() {
    if (BuildConfig.DEBUG) {
        Log.d("Tag", "guarded by a companion DEBUG")
    }
    if (test.logconditional.companion.BuildConfig.DEBUG) Log.d("Tag", "qualified companion DEBUG")
}

open class BaseConfig {
    val DEBUG: Boolean = true
}

class Holder {
    object BuildConfig : BaseConfig()

    fun inheritedGuard() {
        if (BuildConfig.DEBUG) Log.d("Tag", "guarded by an inherited DEBUG")
    }
}
