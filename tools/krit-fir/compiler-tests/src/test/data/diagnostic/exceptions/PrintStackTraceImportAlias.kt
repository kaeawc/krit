// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An import alias of the stdlib function is still printStackTrace().
package test

import kotlin.printStackTrace as dumpTrace

fun aliased() {
    try {
        work()
    } catch (e: Exception) {
        // Go misses this because the call is spelled `dumpTrace`.
        <!PrintStackTrace!>e.dumpTrace()<!>
    }
}

fun work() {}
