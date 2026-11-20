// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Pinned recall difference: Go lets a nested Log class shadow the import file-wide; FIR resolves the top-level call.
package test.logconditional.nestedlog

import android.util.Log

class Holder {
    class Log {
        fun d(tag: String, message: String) {}
    }

    fun nested() {
        Log().d("Tag", compute())
    }
}

fun topLevel() {
    <!LogConditional!>Log.d("Tag", compute())<!>
}

private fun compute(): String = "computed"
