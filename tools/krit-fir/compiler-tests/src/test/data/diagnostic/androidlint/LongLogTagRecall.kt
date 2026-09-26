// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each call below is an android.util.Log level call with
// a tag longer than 23 characters, and FIR reports it. Go misses them: it
// needs the receiver spelled `Log`, so a fully qualified call, an import
// alias, a typealias, and a statically imported level function are other
// spellings; and it
// reads a property's initializer only when it is a bare string literal, so a
// parenthesized one is not resolved.
package test.longlogtag.recall

import android.util.Log as AndroidLog
import android.util.Log.d

typealias Logger = android.util.Log

private val PARENTHESIZED = ("ParenthesizedInitializerTooLong")

fun qualified() {
    <!LongLogTag!>android.util.Log.d("QualifiedCallTagThatIsTooLong", "m")<!>
}

fun importAlias() {
    <!LongLogTag!>AndroidLog.e("AliasedCallTagThatIsTooLongX", "m")<!>
}

fun typeAlias() {
    <!LongLogTag!>Logger.w("TypealiasCallTagThatIsTooLong", "m")<!>
}

fun staticImport() {
    <!LongLogTag!>d("StaticImportTagThatIsTooLong", "m")<!>
}

fun parenthesizedInitializer() {
    <!LongLogTag!>AndroidLog.i(PARENTHESIZED, "m")<!>
}

fun shortTags() {
    android.util.Log.d("Short", "m")
    AndroidLog.e("Short", "m")
    d("Short", "m")
}
