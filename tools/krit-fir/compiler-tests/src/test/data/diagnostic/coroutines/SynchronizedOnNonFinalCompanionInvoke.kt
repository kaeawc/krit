// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 22
// Go matches the call by its written name, so `synchronized(lock) { }` counts
// when `synchronized` is a class whose companion has an `operator fun invoke`:
// the call resolves to `synchronized.Companion.invoke(lock) { }` on the class
// qualifier. FIR reads the written name from that qualifier.
package test

@Suppress("ClassName")
class synchronized private constructor() {
    companion object {
        operator fun invoke(lock: Any, block: () -> Unit) = kotlin.synchronized(lock, block)
    }
}

class CompanionInvoke {
    var lock = Any()
    val fixed = Any()

    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        <!SynchronizedOnNonFinal!>test.synchronized(lock) { }<!>
        synchronized(fixed) { }
    }
}
