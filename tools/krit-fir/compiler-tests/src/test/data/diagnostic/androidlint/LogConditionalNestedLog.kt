// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): the top-level call resolves to android.util.Log through
// the explicit import and is unguarded, so FIR reports it. Go misses it: it
// lets a class named `Log` declared anywhere in the same file (here the
// nested Holder.Log) shadow the import, so it drops every `Log.d` in the
// file. Holder.Log().d is a lookalike that neither reports.
package test.logconditional.nestedlog

import android.util.Log

class Holder {
    class Log {
        fun d(tag: String, message: String) {}
    }

    fun nested() {
        Log().d("Tag", "nested lookalike")
    }
}

fun topLevel() {
    <!LogConditional!>Log.d("Tag", "android.util.Log")<!>
}
