// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Any non-literal-true if condition guards message work in both engines.
package test.logconditional.aliased

import android.util.Log

object GeneratedBuildConfig {
    val DEBUG: Boolean = true
}

typealias BuildConfig = GeneratedBuildConfig

typealias Flags = GeneratedBuildConfig

fun typeAliasGuard() {
    if (BuildConfig.DEBUG) {
        Log.d("Tag", compute())
    }
}

fun otherTypeAlias() {
    if (Flags.DEBUG) {
        Log.d("Tag", compute())
    }
}

private fun compute(): String = "computed"
