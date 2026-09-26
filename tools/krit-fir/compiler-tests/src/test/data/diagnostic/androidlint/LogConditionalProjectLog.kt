// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A project class named Log with level-like methods is not android.util.Log.
// Neither FIR nor Go reports it: without an import of android.util.Log, Go
// does not treat the receiver `Log` as Android's.
package test.logconditional.projectlog

object Log {
    fun d(tag: String, message: String) {}
    fun e(tag: String, message: String) {}
}

class Holder {
    class Log {
        companion object {
            fun w(tag: String, message: String) {}
        }
    }

    fun nested() {
        Log.w("Tag", "nested lookalike")
    }
}

fun projectObject() {
    Log.d("Tag", "project lookalike")
    Log.e("Tag", "project lookalike")
}

fun localClass() {
    class Log {
        fun i(tag: String, message: String) {}
    }
    Log().i("Tag", "local lookalike")
}
