// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves non-const template references and the Java getter and reports them.
// Go reports direct calls and calls that are direct template/concatenation entries; these references are silent.
package test.logconditional.evidence

import android.util.Log
import java.util.Locale
import java.io.File
import java.io.File.separator
import test.logconditional.external.Config
import test.logconditional.external.Config.runtimePrefix

private operator fun String?.plus(other: String): String = this.orEmpty() + other
private operator fun Int.plus(other: String): String = "$this$other"

fun evidence(UPPERCASE: String, locale: Locale) {
    <!LogConditional!>Log.d("Tag", "x$UPPERCASE")<!>
    <!LogConditional!>Log.d("Tag", locale.displayName)<!>
    <!LogConditional!>Log.d("Tag", "x$separator")<!>
    Log.d("Tag", File.separator)
    Log.d("Tag", "x${Integer.MAX_VALUE}")
}

fun ready() { val msg = "ready"; Log.d("Tag", msg + "!") }
fun userDefinedOperator() {
    val msg: String? = null
    Log.d("Tag", msg + "!")
    Log.d("Tag", 1 + "x")
}
// FIR/Lint bind the non-const parameter and report; Go is silent without a message call.
fun failed(msg: String) { <!LogConditional!>Log.d("Tag", msg + "!")<!> }

// FIR/Lint resolve non-const Kotlin members and report; Go is silent without a message call.
fun externalValues() {
    <!LogConditional!>Log.d("Tag", "x${Config.runtimePrefix}")<!>
    <!LogConditional!>Log.d("Tag", "x$runtimePrefix")<!>
    <!LogConditional!>Log.d("Tag", "x" + Config.runtimePrefix)<!>
    <!LogConditional!>Log.d("Tag", "x" + runtimePrefix)<!>
}

// FIR reports resolved non-const bindings in templates/concatenations; Go is silent without calls.
class Repo(private val name: String) {
    fun load() { <!LogConditional!>Log.d("Tag", "load $name")<!> }
}

fun bindings(users: List<String>, pair: Pair<String, String>) {
    val name = "name"
    <!LogConditional!>Log.d("Tag", "load $name")<!>
    users.forEach { user -> <!LogConditional!>Log.d("Tag", "user $user")<!> }
    try {
        println("try")
    } catch (error: Exception) {
        <!LogConditional!>Log.d("Tag", "failed $error")<!>
    }
    val (first, second) = pair
    <!LogConditional!>Log.d("Tag", "x" + first)<!>
    <!LogConditional!>Log.d("Tag", "x$second")<!>
}
