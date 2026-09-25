// RENDER_DIAGNOSTICS_FULL_TEXT
// Go matches the call by its written name, so `synchronized(lock) { }` counts
// when `synchronized` is an object with an `operator fun invoke`: the call
// resolves to `synchronized.invoke(lock) { }` on the object's qualifier. FIR
// reads the written name from that qualifier, bare or qualified.
package test

@Suppress("ClassName")
object synchronized {
    operator fun invoke(lock: Any, block: () -> Unit) = kotlin.synchronized(lock, block)
}

class ObjectInvoke {
    var lock = Any()
    val fixed = Any()

    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        <!SynchronizedOnNonFinal!>test.synchronized(lock) { }<!>
        synchronized(fixed) { }
    }
}
