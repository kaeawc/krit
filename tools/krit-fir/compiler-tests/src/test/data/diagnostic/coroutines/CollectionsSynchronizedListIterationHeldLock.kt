// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 114, 119, 124
// A loop over a val-held wrapper, which Go never reports, is left alone when
// the code around it holds a lock, as Go's rule accepts any enclosing
// synchronized call: a @Synchronized or @GuardedBy function, a
// withLock/read/write block, or a private function whose every caller holds
// a lock. "Without external synchronization" is false there. An inline
// wrapper keeps Go's rule, which only counts calls written synchronized up to
// the nearest named function.
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

    // @Synchronized holds this object's monitor, as synchronized(this) does.
    @Synchronized
    fun annotated() {
        for (listener in listeners) listener.run()
    }

    // @GuardedBy says every caller holds the lock.
    @GuardedBy("lock")
    fun guardedBy() {
        for (listener in listeners) listener.run()
    }

    val count: Int
        @Synchronized get() {
            for (listener in listeners) listener.run()
            return listeners.size
        }

    fun lockBlocks() {
        lock.withLock { for (listener in listeners) listener.run() }
        rwLock.read { for (listener in listeners) listener.run() }
        rwLock.write { for (listener in listeners) listener.run() }
    }

    // Every caller holds a lock.
    private fun dispatchLocked() {
        for (listener in listeners) listener.run()
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
    for (listener in topLevelListeners) listener.run()
}

fun notifyListeners(lock: Any) {
    synchronized(lock) { notifyLocked() }
}

private fun consume(value: Any?) {
    println(value)
}
