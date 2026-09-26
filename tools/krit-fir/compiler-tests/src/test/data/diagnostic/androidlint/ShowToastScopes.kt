// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 43, 53, 73, 87, 124, 139, 144, 149, 163, 171
// Go's evidence, kept as Go reads it. Any enclosing call named `show` counts,
// up to the nearest named function (not across it), whatever it shows; a
// receiverless `show()` anywhere in an enclosing `apply` lambda counts; and a
// variable's `show` is matched by the name it is spelled with, after the
// call, in the enclosing function.
package test.showtoast.scopes

import android.content.Context
import android.widget.Toast

class Banner {
    fun show(toast: Toast) {}
    fun show(block: () -> Unit) {}
    fun show() {}
}

fun show(toast: Toast) {}

fun helperShow(context: Context) {
    show(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
}

fun memberHelperShow(context: Context, banner: Banner) {
    banner.show(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
}

fun inShowLambda(context: Context, banner: Banner) {
    banner.show { Toast.makeText(context, "Hello", Toast.LENGTH_SHORT) }
}

fun inApplyLambda(context: Context, banner: Banner) {
    banner.apply {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
        show()
    }
}

fun acrossNamedFunction(context: Context, banner: Banner) {
    banner.show {
        fun create() {
            <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
        }
        create()
    }
}

fun acrossObjectMethod(context: Context, banner: Banner) {
    banner.show {
        object : Runnable {
            override fun run() {
                <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
            }
        }.run()
    }
}

fun localFunctionShows(context: Context) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    fun display() {
        toast.show()
    }
    display()
}

fun sameNameLater(context: Context, toasts: List<Toast>) {
    val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    toasts.forEach { toast -> toast.show() }
}

fun shownBeforeInOtherFunction(context: Context) {
    val toast = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
    toast.cancel()
}

fun otherFunction(toast: Toast) {
    toast.show()
}

class Stage {
    infix fun show(toast: Toast) {}
}

// An infix call is not a call expression in Go's tree.
fun infixShow(context: Context, stage: Stage) {
    stage show <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
}

fun wrap(toast: Toast): Toast = toast

fun wrappedLocalShown(context: Context) {
    val toast = wrap(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
    toast.show()
}

fun wrappedLocalSafeShown(context: Context) {
    val toast: Toast? = wrap(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
    toast?.show()
}

// Go matches the receiver's last identifier, so another object's `toast`
// counts as this one.
fun wrappedMemberShown(context: Context, screen: ScopesScreen) {
    val toast = wrap(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
    screen.toast.show()
}

class ScopesScreen(val toast: Toast)

fun safeApply(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)?.apply { show() }
}

fun wrappedSafeShow(context: Context) {
    wrap(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))?.show()
}

fun wrappedSafeApply(context: Context) {
    wrap(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))?.apply { show() }
}

fun wrappedSafeApplyWithoutShow(context: Context) {
    wrap(<!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>)?.apply { cancel() }
}

class Marquee {
    fun show(count: Int, block: () -> Any) {}
    fun show(toast: Toast, block: () -> Any) {}
    fun show(block: () -> Any) {}
    fun show() {}
    fun apply(count: Int, block: Marquee.() -> Unit) {}
}

// Go's tree nests `show(args) { .. }`: the lambda belongs to an outer call
// with no name, so a toast in that lambda does not count as shown. A toast
// in the parentheses is under the inner `show(..)` call and does.
fun inShowLambdaWithArguments(context: Context, marquee: Marquee) {
    marquee.show(1) { <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!> }
}

fun inShowLambdaWithNamedArgument(context: Context, marquee: Marquee) {
    marquee.show(count = 2) {
        <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
    }
}

fun inShowLambdaWithEmptyParentheses(context: Context, marquee: Marquee) {
    marquee.show() { <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!> }
}

fun inShowArgumentsWithLambda(context: Context, marquee: Marquee) {
    marquee.show(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)) { 1 }
}

fun inShowParenthesizedLambda(context: Context, marquee: Marquee) {
    marquee.show(1, { Toast.makeText(context, "Hello", Toast.LENGTH_SHORT) })
}

// Likewise the lambda of `apply(args) { .. }` is not an `apply` lambda to Go.
fun inApplyLambdaWithArguments(context: Context, marquee: Marquee) {
    marquee.apply(1) {
        <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>
        show()
    }
}

// A `show` on the name before the call does not count.
fun shownOnlyBefore(context: Context, toasts: List<Toast>) {
    for (toast in toasts) toast.show()
    val toast = wrap(<!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>)
    toast.cancel()
}
