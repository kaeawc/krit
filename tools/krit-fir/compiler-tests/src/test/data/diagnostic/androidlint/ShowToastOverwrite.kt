// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 27, 28, 35, 37, 45, 54, 63, 71, 78, 86, 90, 100, 107
// A toast stored in a variable counts as shown only through a read its value
// can reach: after the store, not in an exclusive `if`/`when` branch, not
// across another write, a jump, or the end of an enclosing loop iteration or
// lambda. A member or top-level property is followed, through the whole file,
// only when the store is its one write (a `null` initializer aside) and is on
// `this`; otherwise it is not followed at all, as in Go (Go does not follow
// assignments). Every toast marked below is reported by Go too, and all but
// the one in `Twice.display()` are never shown. Divergence (precision): the
// unmarked last store in each of the first three functions is shown, so FIR
// does not report it; Go does, since it does not follow assignments.
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
        toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    }

    fun display() {
        toast = <!ShowToast!>Toast.makeText(context, "b", Toast.LENGTH_SHORT)<!>
        toast?.show()
    }
}

class Holder {
    var toast: Toast? = null
}

fun otherInstance(context: Context, a: Holder, b: Holder) {
    a.toast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
    b.toast?.show()
}

var topToast: Toast? = null

fun prepareTop(context: Context) {
    topToast = <!ShowToast!>Toast.makeText(context, "a", Toast.LENGTH_SHORT)<!>
}

fun clearTop() {
    topToast = null
}

fun displayTop() {
    topToast?.show()
}
