// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 189, 193, 203
package test

import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import androidx.compose.runtime.mutableStateOf
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.After
import androidx.annotation.VisibleForTesting
import kotlin.concurrent.withLock

object SafeState {
    val immutable = 0
    // Unlike the task premise, Go does not inspect val Compose/Flow holders.
    val compose = mutableStateOf(0)
    val flow = MutableStateFlow(0)
    @Volatile var published = 0
    var atomic = AtomicInteger(0)
    var map = ConcurrentHashMap<String, Int>()
    var list = CopyOnWriteArrayList<Int>()
    var getterOnly: Int
        get() = 0
        set(_) = Unit
}

object SynchronizedBlock {
    private var count = 0
    fun next(): Int {
        synchronized(this) {
            count++
            return count
        }
    }
}

object SynchronizedMethod {
    private var count = 0
    @Synchronized fun next(): Int = ++count
}

object SynchronizedHelper {
    private var count = 0
    @Synchronized fun next(): Int = increment()
    private fun increment(): Int = ++count
}

object TestHookState {
    private var count = 0
    @VisibleForTesting fun reset() { count = 0 }
}

object NamedHookState {
    private var count = 0
    fun resetForTests() { count++ }
}

object LifecycleState { var count = 0 }
class LifecycleOwner {
    @After fun reset() { LifecycleState.count = 0 }
}

object FinallyState { var value = 0 }
fun resetFinally() {
    try { println("work") } finally { FinallyState.value = 0 }
}

object FakeStore { var state = 0 }
object StubStore { var state = 0 }
object TestStore { var state = 0 }
object StoreFake { var state = 0 }
object StoreTest { var state = 0 }
class FakeOwner { object Nested { var state = 0 } }

class Ordinary {
    var instance = 0
    companion object { var shared = 0 }
    object Nested { val immutable = 0 }
}

val anonymous = object { var state = 0 }
var topLevel = 0
fun locals() { var local = 0; local++ }

// A same-named local factory is harmless here because these are val bindings,
// not because Go or FIR treats its name as a Compose factory.
fun mutableStateOf(value: String): String = value
object LocalFactory { val state = test.mutableStateOf("local") }

object UnwrittenState {
    private var unused = 0
    var endpoint = "x"
        private set
    fun read(): String = endpoint
}

object SafeJdkState {
    @field:Volatile var published = 0
    var long = java.util.concurrent.atomic.AtomicLong(0)
    var boolean = java.util.concurrent.atomic.AtomicBoolean(false)
    var reference = java.util.concurrent.atomic.AtomicReference("x")
    var integers = java.util.concurrent.atomic.AtomicIntegerArray(8)
    var longs = java.util.concurrent.atomic.AtomicLongArray(8)
    var references = java.util.concurrent.atomic.AtomicReferenceArray<String>(8)
    var keys = ConcurrentHashMap.newKeySet<String>()
    var queue = java.util.concurrent.ConcurrentLinkedQueue<Int>()
    var deque = java.util.concurrent.ConcurrentLinkedDeque<Int>()
    var copyList = java.util.concurrent.CopyOnWriteArrayList<Int>()
    var copySet = java.util.concurrent.CopyOnWriteArraySet<Int>()
    var sortedMap = java.util.concurrent.ConcurrentSkipListMap<String, Int>()
    var sortedSet = java.util.concurrent.ConcurrentSkipListSet<Int>()
    var arrayQueue = java.util.concurrent.ArrayBlockingQueue<Int>(8)
    var linkedQueue = java.util.concurrent.LinkedBlockingQueue<Int>()
    var linkedDeque = java.util.concurrent.LinkedBlockingDeque<Int>()
    var priorityQueue = java.util.concurrent.PriorityBlockingQueue<Int>()
    var synchronousQueue = java.util.concurrent.SynchronousQueue<Int>()
    var transferQueue = java.util.concurrent.LinkedTransferQueue<Int>()
    var delayQueue = java.util.concurrent.DelayQueue<java.util.concurrent.Delayed>()
    var longAdder = java.util.concurrent.atomic.LongAdder()
    var doubleAdder = java.util.concurrent.atomic.DoubleAdder()
    var longAccumulator = java.util.concurrent.atomic.LongAccumulator({ left, right -> left + right }, 0)
    var doubleAccumulator = java.util.concurrent.atomic.DoubleAccumulator({ left, right -> left + right }, 0.0)
    var markable: java.util.concurrent.atomic.AtomicMarkableReference<String>? = null
    var stamped: java.util.concurrent.atomic.AtomicStampedReference<String>? = null
    var integerUpdater: java.util.concurrent.atomic.AtomicIntegerFieldUpdater<Any>? = null
    var longUpdater: java.util.concurrent.atomic.AtomicLongFieldUpdater<Any>? = null
    var referenceUpdater: java.util.concurrent.atomic.AtomicReferenceFieldUpdater<Any, String>? = null
}

object SynchronizedLambda {
    private var total = 0
    @Synchronized fun bump(amount: Int) { repeat(amount) { total++ } }
}

object HookLambda {
    private var count = 0
    private val lock = java.util.concurrent.locks.ReentrantLock()
    @VisibleForTesting fun reset() { lock.withLock { count = 0 } }
}

object LifecycleLambda { var count = 0 }
class LifecycleLambdaOwner {
    @After fun tearDown() = kotlinx.coroutines.runBlocking { LifecycleLambda.count = 0 }
}

object PrivateHelperLambda {
    private var count = 0
    @Synchronized fun bump() { repeat(2) { increment() } }
    private fun increment() { repeat(2) { count++ } }
}

object EscapingSynchronizedLambda {
    // FIR, like Go, cannot tell whether a non-inline lambda runs under the lock.
    private var total = 0
    @Synchronized fun launch() { Thread { total++ }.start() }
}

object EscapingPrivateHelperLambda {
    // FIR, like Go, cannot tell whether a non-inline lambda runs under the lock.
    private var total = 0
    @Synchronized fun launch() = incrementLater()
    private fun incrementLater() { Thread { total++ }.start() }
}

object Ids {
    private var nextId = 0
    private val ids = HashMap<String, Int>()
    @Synchronized fun idFor(key: String): Int = ids.computeIfAbsent(key) { nextId++ }
}

object Totals {
    private var total = 0
    @Synchronized fun sum(values: Map<String, Int>) { values.forEach { _, value -> total += value } }
}

object JetBrainsHook {
    private var count = 0
    @org.jetbrains.annotations.VisibleForTesting fun reset() { count = 0 }
}

object ArbitraryHook {
    private var count = 0
    @custom.annotations.VisibleForTesting fun reset() { count = 0 }
}

object LifecycleAfterMethodState {
    // Go's lifecycle list omits AfterMethod; FIR recognizes TestNG's resolved annotation.
    var afterMethodCount = 0
}
object LifecycleBeforeMethodState {
    // Go's lifecycle list omits BeforeMethod; FIR recognizes TestNG's resolved annotation.
    var beforeMethodCount = 0
}
object LifecycleAfterClassState { var afterClassCount = 0 }
class LifecycleTestNgOwner {
    @org.testng.annotations.AfterMethod fun afterMethod() { LifecycleAfterMethodState.afterMethodCount = 0 }
    @org.testng.annotations.BeforeMethod fun beforeMethod() { LifecycleBeforeMethodState.beforeMethodCount = 0 }
    @org.testng.annotations.AfterClass fun afterClass() { LifecycleAfterClassState.afterClassCount = 0 }
}

typealias AliasedAfterTest = org.testng.annotations.AfterTest
object LifecycleAfterTestState { var count = 0 }
class LifecycleTestNgAfterTestOwner {
    @AliasedAfterTest fun tearDown() { LifecycleAfterTestState.count = 0 }
}

annotation class AfterTest
object LocalAnnotationState { var <!MutableStateInObject!>count<!> = 0 }
class LocalAnnotationOwner {
    @AfterTest fun tearDown() { LocalAnnotationState.count = 0 }
}
