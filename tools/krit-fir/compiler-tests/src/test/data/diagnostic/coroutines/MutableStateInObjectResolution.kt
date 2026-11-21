// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 56, 57, 59, 65, 71, 81, 86, 112, 117, 140, 149, 150, 156, 186, 207, 220, 239, 288
package test.resolution

import java.util.concurrent.atomic.AtomicInteger as SafeCounter
import kotlin.jvm.Synchronized as Locked
import kotlin.concurrent.Volatile as Published
import kotlin.synchronized as withMonitor
import org.junit.After as Cleanup
import androidx.annotation.VisibleForTesting
import kotlinx.atomicfu.locks.withLock
import kotlinx.atomicfu.atomic
import kotlin.concurrent.withLock as jdkWithLock

class AtomicInteger(var value: Int)
annotation class Volatile
annotation class Synchronized
annotation class After
fun <T> synchronized(lock: Any, block: () -> T): T = block()

object LookalikeTypes {
    // Go exempts the AtomicInteger substring; this project type is not atomic.
    var <!MutableStateInObject!>atomic<!> = AtomicInteger(0)
    // Go exempts the Volatile substring even in an initializer's string value.
    var <!MutableStateInObject!>text<!> = "Volatile"
    // Go exempts the annotation name; this annotation supplies no visibility guarantee.
    @Volatile var <!MutableStateInObject!>published<!> = 0
}

object LookalikeCall {
    // Go treats any synchronized(...) as a monitor; the project function does not lock.
    private var <!MutableStateInObject!>callCount<!> = 0
    fun next(): Int {
        synchronized(this) {
            callCount++
            return@synchronized callCount
        }
        return 0
    }
}

object LookalikeAnnotation {
    // Go trusts @Synchronized by name; this project annotation does not lock.
    private var <!MutableStateInObject!>annotationCount<!> = 0
    @Synchronized fun next(): Int = ++annotationCount
}

object LookalikeLifecycle {
    // Go trusts @After by name; this is not a test lifecycle annotation.
    var <!MutableStateInObject!>lifecycleCount<!> = 0
    @After fun reset() { lifecycleCount = 0 }
}

object Aliases {
    // Go misses aliased and expanded atomic types; FIR recognizes the resolved class.
    var atomic = SafeCounter(0)
    var typed: SafeAlias? = null
    // Go misses the aliased volatile annotation; FIR recognizes its resolved class.
    @Published var published = 0
}
typealias SafeAlias = SafeCounter

object AliasedMethod {
    // Go misses the aliased synchronized annotation.
    private var methodCount = 0
    @Locked fun next(): Int = ++methodCount
}

object AliasedCall {
    // Go misses the aliased monitor call.
    private var monitorCount = 0
    fun next(): Int = withMonitor(this) { ++monitorCount }
}

open class SafeSubclass : SafeCounter(0) {
    override fun toByte(): Byte = get().toByte()
    override fun toShort(): Short = get().toShort()
}
object InheritedAtomic {
    // Go misses the atomic supertype of the inferred project class.
    var atomic = SafeSubclass()
}

object AliasedLifecycle {
    // Go misses the aliased lifecycle annotation on an external reset.
    var resetCount = 0
}
@Cleanup fun cleanup() { AliasedLifecycle.resetCount = 0 }

object MutatingGetter {
    // Accessor backing-field writes are intentionally ignored as an accepted false negative.
    private var count = 0
        get() { field += 1; return field }
    fun next() = count
}

object NestedWriter {
    // Go intentionally ignores nested declarations; FIR resolves this write to the outer property.
    private var <!MutableStateInObject!>count<!> = 0
    object Writer { fun bump() { NestedWriter.count++ } }
}

object ReadOnlyBackingGetter {
    private var count = 0
        get() = field
    fun read() = count
}

object NestedOwnWrite {
    // A nested declaration's own storage does not write this object's property.
    private var count = 0
    object Writer { private var <!MutableStateInObject!>count<!> = 0; fun bump() { count++ } }
}

object Registry {
    // A distinct name prevents OtherReceiver's finally write from resetting this property.
    var <!MutableStateInObject!>registryCount<!> = 0
}
class OtherReceiver {
    var otherCount = 0
    fun reset() { try { println(otherCount) } finally { this.otherCount = 0 } }
}
object ShadowedReceiverState {
    // Go can mistake a shadowed receiver's finally-write for a reset; FIR resolves its receiver.
    var <!MutableStateInObject!>count<!> = 0
}
class OtherQualifiedReceiver { var count = 0 }
fun resetShadowedReceiver(ShadowedReceiverState: OtherQualifiedReceiver) {
    try { println(ShadowedReceiverState.count) } finally { ShadowedReceiverState.count = 0 }
}

object LockArgumentEvaluation {
    // The conservative synchronized-call exemption includes lock evaluation before the action.
    private var count = 0
    private fun nextLock(): Any { count++; return Any() }
    fun run() { kotlin.synchronized(nextLock()) {} }
}

object LockActionHelper {
    private var count = 0
    private fun bump() { count++ }
    fun run() { kotlin.synchronized(this) { bump() } }
}

object AtomicFuScalars {
    var int = kotlinx.atomicfu.AtomicInt(0)
    var long = kotlinx.atomicfu.AtomicLong(0L)
    var boolean = kotlinx.atomicfu.AtomicBoolean(false)
    var ref = kotlinx.atomicfu.AtomicRef("value")
    var delegated by atomic(0)
}

object AtomicFuArrays {
    // The Go name-only rule is intentionally conservative for these resolved external types.
    var booleanArray = kotlinx.atomicfu.AtomicBooleanArray(4)
    var refArray = kotlinx.atomicfu.AtomicRefArray(4, "value")
}

class AtomicInt
object AtomicFuLookalike {
    // FIR must not treat a local type with an atomicfu short name as safe.
    var <!MutableStateInObject!>counter<!> = AtomicInt()
}

class Box<T>(var value: T)
object UnsafeGenericWrapper {
    // Generic safe-type arguments are conservatively accepted even when wrapper storage is mutable.
    var box = Box(java.util.concurrent.atomic.AtomicInteger(0))
}
object SafeAtomicWrapper {
    var wrapper: java.util.concurrent.atomic.AtomicReference<Box<Int>> =
        java.util.concurrent.atomic.AtomicReference(Box(0))
}

object ShadowedWrite {
    // Go treats a same-named bare local finally write as a reset of this property.
    var <!MutableStateInObject!>shadowCount<!> = 0
    fun localReset() {
        var shadowCount = 1
        try { println(shadowCount) } finally { shadowCount = 0 }
    }
}

object OtherWrite {
    // Another receiver's same-named finally write does not reset this object's property.
    var <!MutableStateInObject!>state<!> = 0
}
class Other { var state = 0 }
fun resetOther(other: Other) {
    try { println(other.state) } finally { other.state = 0 }
}

object PublicPrivateName {
    // Go mistakes the private substring in the name for private visibility; FIR reports public state.
    var <!MutableStateInObject!>privateKey<!> = 0
    fun bump() = kotlin.synchronized(this) { ++privateKey }
}

object LockOnSameLine {
    // Go sees synchronized on the write's line; FIR sees the write outside the monitor.
    private var <!MutableStateInObject!>total<!> = 0
    fun bump() { kotlin.synchronized(this) {}; total++ }
}

object SynchronizedAccessor {
    // Go misses the synchronized getter; FIR honors the accessor without treating it as the named function.
    private var count = 0
    val next: Int
        @kotlin.jvm.Synchronized get() = ++count
}

object AtomicFuMonitor {
    // Go recognizes the synchronized call name; FIR resolves the atomicfu lock. Both are silent.
    private var count = 0
    fun bump() = kotlinx.atomicfu.locks.synchronized(this) { ++count }
}

object AtomicFuLock {
    // Go does not recognize withLock; FIR resolves the atomicfu lock.
    private var count = 0
    private val lock = kotlinx.atomicfu.locks.reentrantLock()
    fun bump() = lock.withLock { ++count }
}

// Same-named writes in unrelated declarations are not writes to Settings.enabled; both checkers stay silent.
object Settings {
    private var enabled = false
    fun isOn() = enabled
    class Toggle(var enabled: Boolean) {
        fun off() { enabled = false }
    }
}

// Go reports the property because it treats += as assignment; FIR resolves plusAssign and sees no variable write.
class Disposable
class CompositeDisposable
operator fun CompositeDisposable.plusAssign(disposable: Disposable) {}
object Tracker {
    private var disposables = CompositeDisposable()
    fun track(disposable: Disposable) { disposables += disposable }
}

// Go recognizes JUnit 5 lifecycle annotations by name; FIR resolves their actual annotation classes.
object AfterAllReset {
    var count = 0
}
class AfterAllHarness {
    companion object {
        @JvmStatic @org.junit.jupiter.api.AfterAll fun reset() { AfterAllReset.count = 0 }
    }
}
object BeforeAllReset {
    var count = 0
}
class BeforeAllHarness {
    companion object {
        @JvmStatic @org.junit.jupiter.api.BeforeAll fun reset() { BeforeAllReset.count = 0 }
    }
}

object Session {
    // Go skips the nested-object write, so it finds no writer for this private property.
    private var <!MutableStateInObject!>token<!>: String? = null
    fun token() = token
    object Login { fun complete(value: String) { token = value } }
}

object HookWithNestedWriter {
    // Go skips the nested-class write and sees only the test-hook reset.
    private var <!MutableStateInObject!>count<!> = 0
    @VisibleForTesting fun reset() { count = 0 }
    class Worker { fun work() { count++ } }
}

object Cache {
    // Go mistakes Buffer.size's bare finally write for a reset of Cache.size.
    var <!MutableStateInObject!>size<!> = 0
}
class Buffer {
    var size = 0
    fun clear() {
        try { println(size) } finally { size = 0 }
    }
}

// Go does not recognize kotlin.concurrent.withLock; FIR resolves the lock call.
object JdkWithLock {
    private var count = 0
    private val lock = java.util.concurrent.locks.ReentrantLock()
    fun bump() = lock.jdkWithLock { ++count }
}
