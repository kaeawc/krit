// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Resource ids: the app's generated R fields are not constants (AGP leaves them non-final), android.R fields are.
package test.maybeconstantresources

object R {
    object id {
        const val button = 1
    }
}

val APP_ID = R.id.button
val APP_ID_SUM = R.id.button + 1
// Go misses the platform ids: it reads a qualified name only when it is a constant of the same file.
val <!MayBeConstant!>PLATFORM_ID<!> = android.R.id.content
val <!MayBeConstant!>PLATFORM_STRING<!> = android.R.string.ok
