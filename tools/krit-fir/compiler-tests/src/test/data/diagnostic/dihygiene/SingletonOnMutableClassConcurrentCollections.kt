// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36, 72, 81, 88, 99, 105, 111, 117
package test

import javax.inject.Singleton
import java.util.Collections
import java.util.concurrent.*
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.snapshots.SnapshotStateMap

// Both are silent: these implementations provide synchronization or snapshot semantics.
@Singleton
class ConcurrentCollections {
    val map = ConcurrentHashMap<String, String>()
    val keys = ConcurrentHashMap.newKeySet<String>()
    val list = CopyOnWriteArrayList<String>()
    val set = CopyOnWriteArraySet<String>()
    val queue = ConcurrentLinkedQueue<String>()
    val deque = ConcurrentLinkedDeque<String>()
    val blockingQueue = LinkedBlockingQueue<String>()
    val blockingDeque = LinkedBlockingDeque<String>()
    val arrayQueue = ArrayBlockingQueue<String>(8)
    val priorityQueue = PriorityBlockingQueue<String>()
    val transferQueue = LinkedTransferQueue<String>()
    val synchronousQueue = SynchronousQueue<String>()
    val sortedMap = ConcurrentSkipListMap<String, String>()
    val sortedSet = ConcurrentSkipListSet<String>()
    val vector = java.util.Vector<String>()
    val table = java.util.Hashtable<String, String>()
    val interfaceType: MutableMap<String, String> = ConcurrentHashMap()
}

// Go reports this public var; FIR exempts the thread-safe map despite its mutable reference.
@Singleton
class StableConcurrentVar {
    var map = ConcurrentHashMap<String, String>()
    fun put() { map["k"] = "v" }
}

// Both are silent: JDK wrappers must not be inferred mutable from their return interface.
@Singleton
class WrappedCollections {
    val list = Collections.synchronizedList(mutableListOf<String>())
    val set = Collections.synchronizedSet(mutableSetOf<String>())
    val map = Collections.synchronizedMap(mutableMapOf<String, String>())
    val collection = Collections.synchronizedCollection(mutableListOf<String>())
    val sortedSet = Collections.synchronizedSortedSet(java.util.TreeSet<String>())
    val sortedMap = Collections.synchronizedSortedMap(java.util.TreeMap<String, String>())
    val readOnlyList = Collections.unmodifiableList(mutableListOf<String>())
    val readOnlySet = Collections.unmodifiableSet(mutableSetOf<String>())
    val readOnlyMap = Collections.unmodifiableMap(mutableMapOf<String, String>())
    val readOnlyCollection = Collections.unmodifiableCollection(mutableListOf<String>())
}

fun snapshotList(): SnapshotStateList<String> = TODO()
fun snapshotMap(): SnapshotStateMap<String, String> = TODO()

// Both are silent: resolved snapshot types, not merely mutable collection interfaces.
@Singleton
class SnapshotCollections {
    val constructedList = SnapshotStateList<String>()
    val constructedMap = SnapshotStateMap<String, String>()
    val factoryList = androidx.compose.runtime.mutableStateListOf<String>()
    val list = snapshotList()
    val map = snapshotMap()
    val interfaceType: MutableList<String> = snapshotList()
}

// Both report reassigned references even when the collection itself is concurrent.
@Singleton
class ConcurrentVar {
    <!SingletonOnMutableClass!>var map = ConcurrentHashMap<String, String>()<!>
    fun reset() { map = ConcurrentHashMap() }
}

class ConcurrentMapSubclass : ConcurrentHashMap<String, String>()

// Both report reassigned references even when concurrency guarantees are inherited.
@Singleton
class ConcurrentSubtypeVar {
    <!SingletonOnMutableClass!>var map = ConcurrentMapSubclass()<!>
    fun reset() { map = ConcurrentMapSubclass() }
}

// Both report reassigned JDK wrapper references.
@Singleton
class WrappedVars {
    <!SingletonOnMutableClass!>var list = Collections.synchronizedList(mutableListOf<String>())<!>
    var map = Collections.unmodifiableMap(mutableMapOf<String, String>())
    fun reset() {
        list = Collections.synchronizedList(mutableListOf())
        map = Collections.unmodifiableMap(mutableMapOf())
    }
}

// Both report reassigned snapshot collection references, not snapshot operations.
@Singleton
class SnapshotListVar {
    <!SingletonOnMutableClass!>var list = SnapshotStateList<String>()<!>
    fun reset() { list = SnapshotStateList() }
}

@Singleton
class SnapshotMapVar {
    <!SingletonOnMutableClass!>var map = SnapshotStateMap<String, String>()<!>
    fun reset() { map = SnapshotStateMap() }
}

@Singleton
class ConcurrentListVar {
    <!SingletonOnMutableClass!>var list = CopyOnWriteArrayList<String>()<!>
    fun reset() { list = CopyOnWriteArrayList() }
}

@Singleton
class UnmodifiableMapVar {
    <!SingletonOnMutableClass!>var map = Collections.unmodifiableMap(mutableMapOf<String, String>())<!>
    fun reset() { map = Collections.unmodifiableMap(mutableMapOf()) }
}
