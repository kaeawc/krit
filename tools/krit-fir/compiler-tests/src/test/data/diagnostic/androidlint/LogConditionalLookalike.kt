// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 23, 27, 31
// Divergence (precision): the file imports android.util.Log, but the receivers
// spelled `Log` below are a local variable, a parameter, and a lambda
// parameter that shadow it, not android.util.Log, so FIR does not report
// them. Go only checks that the receiver is spelled `Log` and that the file
// imports android.util.Log, so it reports each of them.
package test.logconditional.lookalike

import android.util.Log

class FakeLog {
    fun d(tag: String, message: String) {}
    fun e(tag: String, message: String) {}
}

fun localVariable() {
    val Log = FakeLog()
    Log.d("Tag", "local lookalike")
}

fun parameter(Log: FakeLog) {
    Log.e("Tag", "parameter lookalike")
}

fun lambdaParameter(loggers: List<FakeLog>) {
    loggers.forEach { Log -> Log.d("Tag", "lambda parameter lookalike") }
}

fun realLog() {
    <!LogConditional!>Log.d("Tag", "android.util.Log")<!>
}
