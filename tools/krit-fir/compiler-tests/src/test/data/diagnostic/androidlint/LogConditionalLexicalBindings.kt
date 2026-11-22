// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go silences Log calls when any identifier token is not a canonical import or navigation receiver.
package test.logconditional.lexicalbindings

import android.util.Log

private fun compute(): String = "computed"
class FakeLog { fun d(tag: String, message: String) {} }

class LogFailure : Exception() { fun d(tag: String, message: String) {} }

fun catchParameter() {
    try { compute() } catch (Log: LogFailure) { Log.d("T", compute()) }
}

class Screen {
    var logger: FakeLog = FakeLog()
        set(Log) { Log.d("T", compute()); field = Log }

    companion object Log { fun d(tag: String, message: String) {} }
    fun show() { Log.d("T", compute()) }
}

enum class Sink {
    Log;
    fun d(tag: String, message: String) {}
    fun flush() { Log.d("T", compute()) }
}

class Holder { val Log = FakeLog() }
object com { object example { object logging { object Log { fun d(tag: String, message: String) {} } } } }
fun qualifiedReceivers(holder: Holder) {
    holder.Log.d("T", compute())
    com.example.logging.Log.d("T", compute())
}

fun plainAndroidLog() {
    <!LogConditional!>Log.d("T", compute())<!>
}
