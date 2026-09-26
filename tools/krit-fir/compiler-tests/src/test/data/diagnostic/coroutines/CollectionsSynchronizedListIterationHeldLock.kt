// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 109, 114, 119
// Only the wrapper's own monitor protects its iterator. Method annotations,
// other locks, and callers using another lock still leave these loops unsafe.
package test

import java.util.Collections
import java.util.concurrent.locks.ReentrantLock
import java.util.concurrent.locks.ReentrantReadWriteLock
import kotlin.concurrent.read
import kotlin.concurrent.withLock
import kotlin.concurrent.write

annotation class GuardedBy(val value: String)

class Dispatcher {
    private val lock = ReentrantLock()
    private val rwLock = ReentrantReadWriteLock()
    private val listeners = Collections.synchronizedList(mutableListOf<Runnable>())

    // @Synchronized holds this object's monitor, not listeners' monitor.
    @Synchronized
    fun annotated() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    // @GuardedBy names a different lock.
    @GuardedBy("lock")
    fun guardedBy() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    val count: Int
        @Synchronized get() {
            <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
            return listeners.size
        }

    fun lockBlocks() {
        lock.withLock { <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run() }
        rwLock.read { <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run() }
        rwLock.write { <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run() }
    }

    // Some callers hold a different lock.
    private fun dispatchLocked() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    fun dispatch() {
        synchronized(listeners) { dispatchLocked() }
    }

    @Synchronized
    fun dispatchAnnotated() {
        dispatchLocked()
    }

    fun dispatchWithLock() {
        lock.withLock { dispatchLocked() }
    }

    // FIR reports these as findings Go misses: nothing shows the loop holds
    // a lock. One caller holds none.
    private fun sometimesLocked() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    fun callers() {
        synchronized(lock) { sometimesLocked() }
        sometimesLocked()
    }

    // A function reference can be invoked anywhere.
    private fun referenced() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    fun register(): () -> Unit {
        synchronized(lock) { referenced() }
        return ::referenced
    }

    // Callers in other files may hold no lock.
    fun visible() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    fun callVisible() {
        synchronized(lock) { visible() }
    }

    // A function nobody calls shows no caller's lock.
    private fun uncalled() {
        <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
    }

    // The receiver of withLock runs before the lock is taken.
    fun receiverOfWithLock() {
        run {
            <!CollectionsSynchronizedListIteration!>for<!> (listener in listeners) listener.run()
            lock
        }.withLock { }
    }

    // An inline wrapper keeps Go's rule: Go reports these, and so does FIR.
    @Synchronized
    fun inlineAnnotated(values: MutableList<Int>) {
        <!CollectionsSynchronizedListIteration!>for<!> (value in Collections.synchronizedList(values)) consume(value)
    }

    fun inlineWithLock(values: MutableList<Int>) {
        lock.withLock {
            <!CollectionsSynchronizedListIteration!>for<!> (value in Collections.synchronizedList(values)) consume(value)
        }
    }

    private fun inlineLocked(values: MutableList<Int>) {
        <!CollectionsSynchronizedListIteration!>for<!> (value in Collections.synchronizedList(values)) consume(value)
    }

    fun callInlineLocked(values: MutableList<Int>) {
        synchronized(lock) { inlineLocked(values) }
    }
}

// A top-level private function every caller runs under a lock.
private val topLevelListeners = Collections.synchronizedSet(mutableSetOf<Runnable>())

private fun notifyLocked() {
    <!CollectionsSynchronizedListIteration!>for<!> (listener in topLevelListeners) listener.run()
}

fun notifyListeners(lock: Any) {
    synchronized(lock) { notifyLocked() }
}

private fun consume(value: Any?) {
    println(value)
}
