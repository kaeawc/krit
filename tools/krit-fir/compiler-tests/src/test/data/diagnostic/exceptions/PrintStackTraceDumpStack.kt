// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 14, 19, 26
// Thread.dumpStack() prints the current thread's stack trace to the console.
// It is reported on its callee name, like Go and detekt, in or out of a catch.
package test

import java.lang.Thread.dumpStack

fun direct() {
    Thread.<!PrintStackTrace!>dumpStack<!>()
}

fun qualified() {
    java.lang.Thread.<!PrintStackTrace!>dumpStack<!>()
}

fun split() {
    Thread
        .<!PrintStackTrace!>dumpStack<!>()
}

fun inCatch(work: () -> Unit) {
    try {
        work()
    } catch (e: Exception) {
        Thread.<!PrintStackTrace!>dumpStack<!>()
    }
}

// Go misses this because the call has no receiver named Thread.
fun staticImport() {
    <!PrintStackTrace!>dumpStack<!>()
}

class Worker : Thread() {
    // Go misses this because the call has no receiver named Thread.
    fun trace() {
        <!PrintStackTrace!>dumpStack<!>()
    }
}

// A callable reference is not a call, in Go or here.
fun reference(): () -> Unit = Thread::dumpStack
