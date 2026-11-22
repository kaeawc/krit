// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// FIR resolves the final call to Android Log; Go suppresses it because earlier declarations named Log exist in this file.
package test.logconditional.lookalike

import android.util.Log

class FakeLog {
    fun d(tag: String, message: String) {}
    fun e(tag: String, message: String) {}
}

fun localVariable() {
    val Log = FakeLog()
    Log.d("Tag", compute())
}

fun parameter(Log: FakeLog) {
    Log.d("Tag", compute())
}

fun lambdaParameter(loggers: List<FakeLog>) {
    loggers.forEach { Log -> Log.d("Tag", compute()) }
}

fun realLog() {
    // Go's file-wide receiver check intentionally favors avoiding a false positive over this missed call.
    <!LogConditional!>Log.d("Tag", compute())<!>
}

private fun compute(): String = "computed"
