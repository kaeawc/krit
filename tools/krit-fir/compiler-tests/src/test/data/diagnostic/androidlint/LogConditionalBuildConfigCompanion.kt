// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Any non-literal-true if condition guards message work in both engines.
package test.logconditional.companion

import android.util.Log

class BuildConfig {
    companion object {
        const val DEBUG: Boolean = true
    }
}

fun companionGuard() {
    if (BuildConfig.DEBUG) {
        Log.d("Tag", compute())
    }
    if (test.logconditional.companion.BuildConfig.DEBUG) Log.d("Tag", compute())
}

open class BaseConfig {
    val DEBUG: Boolean = true
}

class Holder {
    object BuildConfig : BaseConfig()

    fun inheritedGuard() {
        if (BuildConfig.DEBUG) Log.d("Tag", compute())
    }
}

private fun compute(): String = "computed"
