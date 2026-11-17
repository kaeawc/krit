// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A toast stored in a local variable counts as shown only through a read its
// value can reach: after the store, not in an exclusive `if`/`when` branch,
// not across another write, a jump, or the end of an enclosing loop iteration
// or lambda. Divergence (recall): every marked toast is never shown, and Lint
// reports it; Go reads every assignment as an escape and reports none. A toast
// assigned to a member or top-level property escapes to code that may show it
// (another function, another instance), so neither Go, Lint, nor FIR reports
// the stores in `Twice`, `otherInstance`, and `prepareTop`.
package test.showtoast.overwrite

import android.content.Context
import android.widget.Toast

fun sequential(context: Context) {
    var toast: Toast
    toast = <!ShowToast!>Toast.makeText(context, "first", Toast.LENGTH_SHORT)<!>
    toast = Toast.makeText(context, "second", Toast.LENGTH_SHORT)
    toast.show()
}

fun conditionalThenUnconditional(context: Context, c: Boolean) {
    var toast: Toast? = null
    if (c) toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    toast = Toast.makeText(context, "b", Toast.LENGTH_SHORT)
    toast.show()
}

fun shownOnlyInElse(context: Context, c: Boolean) {
    var toast: Toast
    if (c) {
        toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    } else {
        toast = Toast.makeText(context, "b", Toast.LENGTH_SHORT)
        toast.show()
    }
}

fun elseShowsPrevious(context: Context, c: Boolean) {
    var toast: Toast? = null
    if (c) {
        toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    } else {
        toast?.show()
    }
}

fun returnsBeforeShow(context: Context, c: Boolean) {
    var toast: Toast? = null
    if (c) {
        toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
        return
    }
    toast?.show()
}

fun overwrittenInLoop(context: Context, items: List<String>) {
    var toast: Toast? = null
    for (item in items) {
        toast = <!ShowToast!>Toast.makeText(context, item, Toast.LENGTH_SHORT)<!>
    }
    toast?.show()
}

fun overwrittenByCapturedWrite(context: Context) {
    var toast: Toast? = null
    val reset = { toast = null }
    toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    reset()
    toast?.show()
}

fun storedInLambda(context: Context, post: (() -> Unit) -> Unit) {
    var toast: Toast? = null
    post { toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!> }
    toast?.show()
}

class Twice(private val context: Context) {
    private var toast: Toast? = null

    fun prepare() {
        toast = Toast.makeText(context, "a", Toast.LENGTH_SHORT)
    }

    fun display() {
        toast = Toast.makeText(context, "b", Toast.LENGTH_SHORT)
        toast?.show()
    }
}

class Holder {
    var toast: Toast? = null
}

fun otherInstance(context: Context, a: Holder, b: Holder) {
    a.toast = Toast.makeText(context, "a", Toast.LENGTH_SHORT)
    b.toast?.show()
}

var topToast: Toast? = null

fun prepareTop(context: Context) {
    topToast = Toast.makeText(context, "a", Toast.LENGTH_SHORT)
}

fun clearTop() {
    topToast = null
}

fun displayTop() {
    topToast?.show()
}
