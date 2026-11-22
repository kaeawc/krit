// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go conservatively skips the top-level receiver because a nested Log class is declared in this file; FIR resolves it.
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
    // This FIR finding is intentionally silent in Go under the file-wide shadow policy.
    <!LogConditional!>Log.d("Tag", compute())<!>
}

private fun compute(): String = "computed"
