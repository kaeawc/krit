// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 13, 17, 21, 26, 29, 32, 36, 40, 44, 49, 55, 59, 68
package test.showtoast

import android.content.Context
import android.widget.Toast

private lateinit var appContext: Context

val topLevelToast: Toast = <!ShowToast!>Toast.makeText(appContext, "Top", Toast.LENGTH_SHORT)<!>

fun bare(context: Context) {
    <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
}

fun resource(context: Context) {
    <!ShowToast!>Toast.makeText(context, 1, Toast.LENGTH_LONG)<!>
}

fun multiline(context: Context) {
    <!ShowToast!>Toast<!>
        .makeText(context, "Hello", Toast.LENGTH_SHORT)
}

fun returned(context: Context): Toast {
    return <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
}

fun expressionBody(context: Context) = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>

fun passed(context: Context, sink: (Toast) -> Unit) {
    sink(<!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>)
}

fun collected(context: Context, toasts: MutableList<Toast>) {
    toasts.add(<!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>)
}

fun configuredOnly(context: Context) {
    <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>.apply { setDuration(Toast.LENGTH_LONG) }
}

fun localNeverShown(context: Context) {
    val toast = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
    toast.cancel()
}

fun otherLocalShown(context: Context, other: Toast) {
    val toast = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
    other.show()
    toast.cancel()
}

fun inLambda(context: Context, run: (() -> Unit) -> Unit) {
    run { <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!> }
}

class MemberNeverShown(context: Context) {
    private val toast = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>

    fun cancel() {
        toast.cancel()
    }
}

object Holder {
    lateinit var context: Context
    val toast: Toast by lazy { <!ShowToast!>Toast.makeText(context, "Lazy", Toast.LENGTH_SHORT)<!> }
}

fun shownChain(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
}

fun shownSafeChain(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)?.show()
}

fun shownAfterApply(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).apply { setDuration(Toast.LENGTH_LONG) }.show()
}

fun shownInApply(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).apply {
        setDuration(Toast.LENGTH_LONG)
        show()
    }
}

fun shownInApplyWithThis(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).apply { this.show() }
}

fun shownLocal(context: Context) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast.setDuration(Toast.LENGTH_LONG)
    toast.show()
}

fun shownLocalSafe(context: Context) {
    val toast: Toast? = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast?.show()
}

fun shownLocalConfigured(context: Context) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).apply { setText("Configured") }
    toast.show()
}

fun shownInsideLambda(context: Context, post: (() -> Unit) -> Unit) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    post { toast.show() }
}

class MemberShown(context: Context) {
    private val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    private val other = Toast.makeText(context, "Other", Toast.LENGTH_SHORT)

    fun display() {
        toast.show()
        this.other.show()
    }
}

class CompanionShown {
    companion object {
        lateinit var context: Context
        val toast: Toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }

    fun display() {
        toast.show()
    }
}

fun anonymousObject(context: Context): Runnable {
    val runnable = object : Runnable {
        val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)

        override fun run() {
            toast.show()
        }
    }
    return runnable
}
