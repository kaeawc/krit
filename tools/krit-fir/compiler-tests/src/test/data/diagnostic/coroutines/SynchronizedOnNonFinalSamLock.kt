// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 28
// A `synchronized` lookalike whose parameter is a Kotlin `fun interface`: the
// function-typed `var` argument reaches FIR wrapped in a SAM conversion. Go
// reports the bare name when the class declares it as a `var`, and the lock
// is that `var`, so FIR reports it too.
package test

fun interface Locker {
    fun lock()
}

fun synchronized(locker: Locker) = locker.lock()

class SamLock {
    var fn: () -> Unit = {}
    val fixed: () -> Unit = {}

    fun sam() {
        <!SynchronizedOnNonFinal!>synchronized(fn)<!>
        synchronized(fixed)
    }

    // A local var smart-cast to non-null, then SAM-converted.
    fun smartCastSam() {
        var local: (() -> Unit)? = null
        local = {}
        <!SynchronizedOnNonFinal!>synchronized(local)<!>
    }
}
