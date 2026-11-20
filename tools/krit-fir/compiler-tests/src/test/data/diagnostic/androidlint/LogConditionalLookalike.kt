// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 27
// Receiver shadowing is not Android logging. Only the unshadowed call performs Android log work.
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
    <!LogConditional!>Log.d("Tag", compute())<!>
}

private fun compute(): String = "computed"
