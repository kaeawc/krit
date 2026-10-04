// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 27, 31, 35, 39, 43, 47, 54, 58, 63, 77, 82, 88, 99, 111, 119, 123, 131, 137, 145, 154, 156, 163, 170, 178, 187, 193
// Divergence (precision): every toast below is shown, so the message ("called
// without .show()") is false of the code and FIR does not report it. Go
// reports each one: it only sees a `show` call that encloses the makeText
// call, a receiverless `show()` in an enclosing `apply` lambda, or a `show`
// call on the bare name of the property the call initializes, in the same
// function (or class, for a member without an enclosing function). A toast
// shown in the lambda of another scope function, through `!!` or a cast,
// through another variable, after an assignment (also of an `if` or elvis
// holding the call), as a parameter's default value, from outside that
// function or class, or in a getter or top-level lambda is shown all the same.
package test.showtoast.divergence

import android.content.Context
import android.widget.Toast

private lateinit var appContext: Context

val topLevel: Toast = Toast.makeText(appContext, "Top", Toast.LENGTH_SHORT)

fun showTopLevel() {
    topLevel.show()
}

fun alsoIt(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).also { it.show() }
}

fun alsoNamed(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).also { toast -> toast.show() }
}

fun letIt(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).let { it.show() }
}

fun safeLet(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)?.let { it.show() }
}

fun runReceiver(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).run { show() }
}

fun withReceiver(context: Context) {
    with(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)) {
        setDuration(Toast.LENGTH_LONG)
        show()
    }
}

fun alsoThenLet(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).also { it.setDuration(Toast.LENGTH_LONG) }.let { it.show() }
}

fun variableApply(context: Context) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast.apply { show() }
}

class NotNullAssertion(context: Context) {
    private var toast: Toast? = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)

    fun display() {
        toast!!.show()
    }
}

fun variableCast(context: Context) {
    val toast: Any = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    (toast as Toast).show()
}

fun assigned(context: Context) {
    var toast: Toast? = null
    toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast.show()
}

fun copied(context: Context) {
    val first = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    val second = first
    second.show()
}

class Screen(context: Context) {
    val toast: Toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
}

fun showScreen(screen: Screen) {
    screen.toast.show()
}

class Field {
    private var toast: Toast? = null

    fun create(context: Context) {
        toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }

    fun display() {
        toast?.show()
    }
}

class InitAssigned(context: Context) {
    private val toast: Toast

    init {
        toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }

    fun display() {
        toast.show()
    }
}

fun defaultParameter(context: Context, toast: Toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)) {
    toast.show()
}

class ConstructorDefault(context: Context, val toast: Toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)) {
    fun display() {
        toast.show()
    }
}

fun assignedFromIf(context: Context, enabled: Boolean) {
    var toast: Toast? = null
    toast = if (enabled) Toast.makeText(context, "Hello", Toast.LENGTH_SHORT) else null
    toast?.show()
}

// Go does not read a `when` subject variable as a property.
fun whenSubject(context: Context) {
    when (val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)) {
        null -> {}
        else -> toast.show()
    }
}

fun assignedFromElvis(context: Context, cached: Toast?) {
    var toast: Toast? = null
    toast = cached ?: Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast.show()
}

// A toast stored again after it is shown is shown too, and so is one stored
// in a branch that falls through to the `show`, or in a loop body that shows
// it before the next store.
fun overwrittenAfterShow(context: Context) {
    var toast: Toast
    toast = Toast.makeText(context, "a", Toast.LENGTH_SHORT)
    toast.show()
    toast = Toast.makeText(context, "b", Toast.LENGTH_SHORT)
    toast.show()
}

fun shownAfterBranch(context: Context, c: Boolean) {
    var toast: Toast? = null
    if (c) {
        toast = Toast.makeText(context, "a", Toast.LENGTH_SHORT)
    }
    toast?.show()
}

fun shownAfterLambda(context: Context, items: List<String>) {
    var toast: Toast? = null
    toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    items.forEach { if (it.isEmpty()) return@forEach }
    toast.show()
}

fun shownInLoop(context: Context, items: List<String>) {
    var toast: Toast
    for (item in items) {
        toast = Toast.makeText(context, item, Toast.LENGTH_SHORT)
        toast.show()
    }
}

// Go reads a variable's `show` only in an enclosing named function or class,
// so it reports a local shown in a property getter or in a top-level lambda.
val getterToast: Toast
    get() {
        val toast = Toast.makeText(appContext, "Hello", Toast.LENGTH_SHORT)
        toast.show()
        return toast
    }

val handler = { context: Context ->
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toast.show()
}
