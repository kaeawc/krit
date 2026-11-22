// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 27, 36, 45, 54, 64, 74, 84, 94, 112, 121, 129
package test

import javax.inject.Singleton
import kotlin.concurrent.withLock
import kotlin.concurrent.read
import kotlin.concurrent.write
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.atomicfu.locks.withLock as atomicWithLock

// Deliberate Go parity for vars: ANY package's GuardedBy is accepted, even a project annotation.
// FIR additionally protects collection vals, which Go still reports.
annotation class GuardedBy(val value: String)

@Singleton
class GuardedState {
    @GuardedBy("this") private var value = 0
    @GuardedBy("this") private val entries = mutableListOf<String>()
    fun bump() { value++; entries.add("entry") }
}

// FIR exempts private setters written only in synchronized methods; Go reports this shape.
@Singleton
class PrivateSetter {
    var value = 0
        private set
    @Synchronized fun bump() { value++ }
    fun read() = value
}

// FIR is deliberately quieter: Go reports collection vals despite synchronized-only access.
@Singleton
class SynchronizedCollection {
    private val entries = mutableListOf<String>()
    @Synchronized fun add() { entries.add("entry") }
    @Synchronized fun read() = entries.size
}

// FIR is deliberately quieter: Go reports collection vals despite synchronized(lock) access.
@Singleton
class LockedCollection {
    private val lock = Any()
    private val entries = mutableListOf<String>()
    fun add() { synchronized(lock) { entries.add("entry") } }
    fun read() = synchronized(lock) { entries.size }
}

// FIR proves Mutex.withLock protection; Go does not recognize this wrapper.
@Singleton
class MutexState {
    private val mutex = Mutex()
    private var value = 0
    private val entries = mutableListOf<String>()
    suspend fun bump() { mutex.withLock { value++; entries.add("entry") } }
    suspend fun read() = mutex.withLock { value + entries.size }
}

// FIR proves kotlin.concurrent.withLock protection; Go does not recognize this wrapper.
@Singleton
class ReentrantState {
    private val lock: java.util.concurrent.locks.Lock = java.util.concurrent.locks.ReentrantLock()
    private var value = 0
    private val entries = mutableListOf<String>()
    fun bump() { lock.withLock { value++; entries.add("entry") } }
    fun read() = lock.withLock { value + entries.size }
}

// FIR proves read/write lock protection; Go does not recognize these wrappers.
@Singleton
class ReadWriteState {
    private val lock = java.util.concurrent.locks.ReentrantReadWriteLock()
    private var value = 0
    private val entries = mutableListOf<String>()
    fun bump() { lock.write { value++; entries.add("entry") } }
    fun read() = lock.read { value + entries.size }
}

// FIR exempts unwritten private/private-set vars, not the externally writable public var.
// Go anchors its finding on privateValue; FIR anchors on publicValue.
@Singleton
class Unwritten {
    private var privateValue = 0
    <!SingletonOnMutableClass!>var publicValue = 0<!>
    var privateSetter = 0
        private set
    fun read() = privateValue + publicValue + privateSetter
}

// FIR is deliberately quieter: Go reports an init-only private-set var.
@Singleton
class InitializationOnly {
    var value: Int
        private set
    init { value = 1 }
    fun read() = value
}

// Both exempt atomicfu's synchronized (resolved to its real package).
@Singleton
class AtomicfuSynchronized : kotlinx.atomicfu.locks.SynchronizedObject() {
    private var value = 0
    fun bump() { kotlinx.atomicfu.locks.synchronized(this) { value++ } }
    fun read() = kotlinx.atomicfu.locks.synchronized(this) { value }
}

// FIR proves atomicfu's reentrantLock/withLock protection; Go does not recognize withLock.
@Singleton
class AtomicfuReentrant {
    private val lock = kotlinx.atomicfu.locks.reentrantLock()
    private var value = 0
    fun bump() { lock.atomicWithLock { value++ } }
    fun read() = lock.atomicWithLock { value }
}

// Go mistakes the shadowing parameter for an unsynchronized member read.
// FIR sees only the synchronized accesses to the member and drops the finding.
@Singleton
class ShadowedReference {
    private var value = 0
    @Synchronized fun bump() { value++ }
    fun unrelated(value: Int) = value
}

// Public, internal and protected setters can be written outside this class.
@Singleton
open class ExternalWriters {
    <!SingletonOnMutableClass!>internal var internalValue = 0<!>
    protected var protectedValue = 0
}
