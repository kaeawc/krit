// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 25, 26
// Positive: a monitor-lock wrapper whose lock parameter is a type parameter
// bounded by Any (or Any?) still locks on the argument, so a boxed-primitive
// lock must trigger SynchronizedOnBoxedPrimitive, as it does in Go.
package test

fun <T : Any, R> synchronized(lock: T, block: () -> R): R = kotlin.synchronized(lock, block)

object NullableBound {
    fun <T, R> synchronized(lock: T, block: () -> R): R = kotlin.synchronized(lock as Any, block)

    val version: Long = 1L

    fun use() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(version) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(2) { }<!>
    }
}

class GenericLock {
    val count: Int = 1

    fun use() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1) { }<!>
    }
}
