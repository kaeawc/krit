// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 21, 26, 29, 31, 33, 35, 38, 41, 43, 45, 48, 54, 58, 61, 64, 70
// Positives for WorldReadableFiles: every use of the platform constant
// android.content.Context.MODE_WORLD_READABLE, however it is reached
// (qualified, fully qualified, through a Context subclass such as Activity,
// bare inside a subclass or its companion, through a typealias, in a
// callable reference, an annotation argument, a default value, a comparison,
// a `when` branch, or a string template), and inside lambdas and object
// expressions. The finding sits on the MODE_WORLD_READABLE name, the
// identifier Go reports, so a use split across lines reports on the name's
// line like Go.
package test

import android.app.Activity
import android.content.Context
import java.io.FileOutputStream

class Prefs(private val context: Context) {
    fun prefs() = context.getSharedPreferences("data", Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

    fun combined(): FileOutputStream = context.openFileOutput("f", Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!> or Context.MODE_PRIVATE)

    fun multiline() = context.getSharedPreferences(
        "data",
        Context
            .<!WorldReadableFiles!>MODE_WORLD_READABLE<!>,
    )

    fun qualified() = context.getSharedPreferences("data", android.content.Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

    fun defaultArg(mode: Int = Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>) = context.getSharedPreferences("data", mode)

    fun compare(mode: Int) = mode == Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>

    fun template() = "mode=${Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>}"
}

const val WORLD = Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>

class Main : Activity() {
    fun bare() = getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

    fun viaSubclass() = getSharedPreferences("data", Activity.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

    fun inLambda() = run { openFileOutput("f", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>) }

    val anonymous = object {
        fun mode() = Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>
    }
}

typealias Ctx = Context

fun aliased() = Ctx.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>

annotation class FileMode(val value: Int)

@FileMode(Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>)
fun annotated() = Unit

fun reference() = Context::<!WorldReadableFiles!>MODE_WORLD_READABLE<!>

fun branch(mode: Int) = when (mode) {
    Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!> -> "world"
    else -> "other"
}

class Companioned : Activity() {
    companion object {
        val MODE = <!WorldReadableFiles!>MODE_WORLD_READABLE<!>
    }
}
