// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// A project class named Thread shadows java.lang.Thread.
package test

class Thread {
    companion object {
        fun dumpStack() {}
    }
}

// Go reports this because the receiver is named Thread; FIR is correct to drop
// it because test.Thread.dumpStack() prints nothing.
fun lookalike() {
    Thread.dumpStack()
}
