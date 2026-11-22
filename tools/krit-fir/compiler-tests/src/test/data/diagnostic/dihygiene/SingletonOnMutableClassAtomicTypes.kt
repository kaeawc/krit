// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 53
@file:OptIn(kotlin.concurrent.atomics.ExperimentalAtomicApi::class)
package test

import javax.inject.Singleton
import kotlinx.atomicfu.atomic
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.MutableSharedFlow

// Both exempt atomic delegates, even though the property's own type is Int.
@Singleton
class AtomicDelegate {
    private var value by atomic(0)
    private var total by atomic(0L)
    private var enabled by atomic(false)
    private var reference by atomic("value")
    fun bump() { value++; total++; enabled = true; reference = "next" }
}

// Go exempts mutex/hot-flow text markers; FIR reports unsafe reference reassignment.
@Singleton
class CoroutinePrimitives {
    <!SingletonOnMutableClass!>var mutex: Mutex = Mutex()<!>
    var state: MutableStateFlow<Int> = MutableStateFlow(0)
    var events: MutableSharedFlow<Int> = MutableSharedFlow()
    fun reset() { mutex = Mutex(); state = MutableStateFlow(1); events = MutableSharedFlow() }
}

@Singleton
class ReassignedStateFlow {
    <!SingletonOnMutableClass!>var state = MutableStateFlow(0)<!>
    fun reset() { state = MutableStateFlow(1) }
}

@Singleton
class ReassignedSharedFlow {
    <!SingletonOnMutableClass!>var events = MutableSharedFlow<Int>()<!>
    fun reset() { events = MutableSharedFlow() }
}

@Singleton
class StableCoroutinePrimitives {
    var mutex = Mutex()
    var state = MutableStateFlow(0)
    fun use() { mutex.tryLock(); state.value = 1 }
}

// Both report reassigned atomicfu references: an atomic object's safety does not protect its var.
@Singleton
class AtomicfuTypes {
    <!SingletonOnMutableClass!>var count = atomic(0)<!>
    var total = atomic(0L)
    var enabled = atomic(false)
    var reference = atomic("value")
    fun reset() { count = atomic(1); total = atomic(1L); enabled = atomic(true); reference = atomic("next") }
}

// Go exempts atomic type markers; FIR reports reassigned atomic references.
@Singleton
class KotlinAtomicTypes {
    <!SingletonOnMutableClass!>var count = kotlin.concurrent.atomics.AtomicInt(0)<!>
    var total = kotlin.concurrent.atomics.AtomicLong(0L)
    var enabled = kotlin.concurrent.atomics.AtomicBoolean(false)
    var reference = kotlin.concurrent.atomics.AtomicReference("value")
    var counts = kotlin.concurrent.atomics.AtomicIntArray(2)
    var totals = kotlin.concurrent.atomics.AtomicLongArray(2)
    var references = kotlin.concurrent.atomics.AtomicArray(2) { "value" }
    fun reset() {
        count = kotlin.concurrent.atomics.AtomicInt(1)
        total = kotlin.concurrent.atomics.AtomicLong(1L)
        enabled = kotlin.concurrent.atomics.AtomicBoolean(true)
        reference = kotlin.concurrent.atomics.AtomicReference("next")
        counts = kotlin.concurrent.atomics.AtomicIntArray(3)
        totals = kotlin.concurrent.atomics.AtomicLongArray(3)
        references = kotlin.concurrent.atomics.AtomicArray(3) { "next" }
    }
}

// Both exempt android.os.Handler constructor confinement.
@Singleton
class HandlerConfined(private val handler: android.os.Handler) {
    private var value = 0
    fun bump() { handler.post { value++ } }
}
