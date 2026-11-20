// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike owners never report, even when their messages perform work.
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
        Log.w("Tag", compute())
    }
}

fun projectObject() {
    Log.d("Tag", compute())
    Log.e("Tag", compute())
}

fun localClass() {
    class Log {
        fun i(tag: String, message: String) {}
    }
    Log().i("Tag", compute())
}

private fun compute(): String = "computed"
