// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 32, 52, 60, 70, 76
package test

import javax.inject.Singleton

@javax.inject.Qualifier
annotation class IoDispatcher

// Go's main/UI name list applies to dispatcher and executor types without a qualifier.
@Singleton
class NamedMain(private val mainDispatcher: kotlinx.coroutines.CoroutineDispatcher) {
    private var value = 0
    fun bump() { value++ }
}

@Singleton
class NamedUiExecutor(private val uiDispatcher: java.util.concurrent.Executor) {
    private var value = 0
    fun bump() { value++ }
}

// Arbitrary qualifiers cannot establish single-thread confinement.
@Singleton
class IoConfined(@IoDispatcher private val io: kotlinx.coroutines.CoroutineDispatcher) {
    <!SingletonOnMutableClass!>private var value = 0<!>
    fun bump() { value++ }
}

@Singleton
class BackgroundExecutor(@javax.inject.Named("bg") private val executor: java.util.concurrent.ExecutorService) {
    <!SingletonOnMutableClass!>private var value = 0<!>
    fun bump() { value++ }
}

class SyncList : ArrayList<String>() {
    @Synchronized override fun add(element: String): Boolean = super.add(element)
}

fun build(): ArrayList<String> = SyncList()

// Neither declared nor factory return types prove the actual implementation unprotected.
@Singleton
class SubclassCollections {
    val direct: ArrayList<String> = SyncList()
    val returned = build()
}

// Non-collection concurrent-package types do not protect reassignment of their references.
@Singleton
class ConcurrentReferences {
    <!SingletonOnMutableClass!>private var future: java.util.concurrent.ScheduledFuture<*>? = null<!>
    fun reset(nextFuture: java.util.concurrent.ScheduledFuture<*>) {
        future = nextFuture
    }
}

@Singleton
class ExecutorReference {
    <!SingletonOnMutableClass!>private var executor: java.util.concurrent.ExecutorService? = null<!>
    fun reset(nextExecutor: java.util.concurrent.ExecutorService) {
        executor = nextExecutor
    }
}

// FIR exempts stable atomic/lock own types; Go still reports the lock var.
@Singleton
class StableReferences {
    var counter = java.util.concurrent.atomic.AtomicInteger(0)
    var lock = java.util.concurrent.locks.ReentrantLock()
    fun bump() { counter.incrementAndGet(); lock.lock(); lock.unlock() }
}

@Singleton
class ReassignedReferences {
    <!SingletonOnMutableClass!>var lock = java.util.concurrent.locks.ReentrantLock()<!>
    var counter = java.util.concurrent.atomic.AtomicInteger(0)
    fun reset() { lock = java.util.concurrent.locks.ReentrantLock(); counter = java.util.concurrent.atomic.AtomicInteger(1) }
}
