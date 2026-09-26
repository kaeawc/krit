// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: an import alias of a monitor-lock function is not a call named
// synchronized(), which is how the Go rule recognizes the call, so it must
// not trigger SynchronizedOnBoxedPrimitive.
package test

import test.synchronized as locked

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class AliasedLock {
    val count: Int = 1

    fun use() {
        locked(count) { }
        locked(1) { }
    }
}
