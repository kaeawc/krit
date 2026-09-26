// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27
// A condition written `BuildConfig.DEBUG` guards the call when BuildConfig is
// a typealias, as Go matches the condition text `BuildConfig.DEBUG`. FIR
// matches. A typealias with another name (`Flags.DEBUG`) is not a guard for
// either (see the go-lines header).
package test.logconditional.aliased

import android.util.Log

object AppBuildConfig {
    val DEBUG: Boolean = true
}

typealias BuildConfig = AppBuildConfig

typealias Flags = AppBuildConfig

fun typeAliasGuard() {
    if (BuildConfig.DEBUG) {
        Log.d("Tag", "guarded through a typealias spelled BuildConfig")
    }
}

fun otherTypeAlias() {
    if (Flags.DEBUG) {
        <!LogConditional!>Log.d("Tag", "DEBUG through a typealias with another name")<!>
    }
}
