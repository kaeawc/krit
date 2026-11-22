package stubs

import kotlinx.atomicfu.locks.*

class AtomicLocksSmoke : SynchronizedObject() {
    private val lock = reentrantLock()

    fun exercise(): Int = synchronized(this) {
        lock.withLock { 1 }
    }
}
