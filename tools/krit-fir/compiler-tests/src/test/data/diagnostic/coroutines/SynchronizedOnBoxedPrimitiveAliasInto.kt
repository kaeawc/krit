// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: an import alias INTO the name synchronized. The Go rule matches
// the call by its written name, and the aliased function is a monitor lock
// (its first parameter is Any), so this is a real boxed-primitive lock and
// must trigger SynchronizedOnBoxedPrimitive.
package test

import test.lockOn as synchronized

fun <R> lockOn(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class AliasedIntoSynchronized {
    val count: Int = 1

    fun use() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1) { }<!>
    }
}
