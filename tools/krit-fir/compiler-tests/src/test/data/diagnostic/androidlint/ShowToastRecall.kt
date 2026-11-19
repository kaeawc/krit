// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each call below is android.widget.Toast.makeText, never
// shown, and FIR reports it. Go misses them: it needs the receiver spelled
// `Toast`, so a fully qualified call, an import alias, a typealias, and a
// statically imported makeText are other spellings.
package test.showtoast.recall

import android.content.Context
import android.widget.Toast as AndroidToast
import android.widget.Toast.makeText

typealias Snack = android.widget.Toast

fun qualified(context: Context) {
    <!ShowToast!>android.widget.Toast.makeText(context, "Hello", AndroidToast.LENGTH_SHORT)<!>
}

fun importAlias(context: Context) {
    <!ShowToast!>AndroidToast.makeText(context, "Hello", AndroidToast.LENGTH_SHORT)<!>
}

fun typeAlias(context: Context) {
    <!ShowToast!>Snack.makeText(context, "Hello", AndroidToast.LENGTH_SHORT)<!>
}

fun staticImport(context: Context) {
    <!ShowToast!>makeText(context, "Hello", AndroidToast.LENGTH_SHORT)<!>
}

fun shownSpellings(context: Context) {
    android.widget.Toast.makeText(context, "Hello", AndroidToast.LENGTH_SHORT).show()
    AndroidToast.makeText(context, "Hello", AndroidToast.LENGTH_SHORT).show()
    makeText(context, "Hello", AndroidToast.LENGTH_SHORT).show()
}
