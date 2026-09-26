// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 26, 34, 46
// Go matches the call by its written name, so any call written
// `synchronized(...)` counts: a same-package wrapper that shadows
// kotlin.synchronized, a member function, and a value named synchronized
// called through `invoke`. FIR matches the same way.
package test

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class Locker {
    operator fun <R> invoke(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)
}

class Guarded {
    private var lock = Any()
    private val fixed = Any()

    fun wrapper() {
        <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        synchronized(fixed) { }
    }

    fun invokeConvention() {
        val synchronized = Locker()
        <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
        synchronized(fixed) { }
    }

    // The value is smart-cast to Locker before it is invoked.
    fun smartCastInvoke(candidate: Any) {
        val synchronized = candidate
        if (synchronized is Locker) {
            <!SynchronizedOnNonFinal!>synchronized(lock) { }<!>
            synchronized(fixed) { }
        }
    }
}

class MemberLookalike {
    private var count = 1

    fun <R> synchronized(times: Int, block: () -> R): R = block()

    fun use() {
        <!SynchronizedOnNonFinal!>synchronized(count) { }<!>
    }
}
