package androidx.compose.runtime.snapshots

interface StateObject {
    val firstStateRecord: StateRecord
    fun prependStateRecord(value: StateRecord)
}

abstract class StateRecord {
    abstract fun assign(value: StateRecord)
    abstract fun create(): StateRecord
}

class SnapshotStateList<T> : StateObject, MutableList<T>, java.util.RandomAccess {
    override val firstStateRecord: StateRecord get() = TODO()
    override fun prependStateRecord(value: StateRecord): Unit = TODO()
    override val size: Int get() = TODO()
    override fun isEmpty(): Boolean = TODO()
    override fun contains(element: T): Boolean = TODO()
    override fun containsAll(elements: Collection<T>): Boolean = TODO()
    override fun iterator(): MutableIterator<T> = TODO()
    override fun listIterator(): MutableListIterator<T> = TODO()
    override fun listIterator(index: Int): MutableListIterator<T> = TODO()
    override fun indexOf(element: T): Int = TODO()
    override fun lastIndexOf(element: T): Int = TODO()
    override fun subList(fromIndex: Int, toIndex: Int): MutableList<T> = TODO()
    override fun add(element: T): Boolean = TODO()
    override fun addAll(elements: Collection<T>): Boolean = TODO()
    override fun addAll(index: Int, elements: Collection<T>): Boolean = TODO()
    override fun clear(): Unit = TODO()
    override fun remove(element: T): Boolean = TODO()
    override fun removeAll(elements: Collection<T>): Boolean = TODO()
    override fun retainAll(elements: Collection<T>): Boolean = TODO()
    override fun get(index: Int): T = TODO()
    override fun add(index: Int, element: T): Unit = TODO()
    override fun removeAt(index: Int): T = TODO()
    override fun set(index: Int, element: T): T = TODO()
}

class SnapshotStateMap<K, V> : StateObject, MutableMap<K, V> {
    override val firstStateRecord: StateRecord get() = TODO()
    override fun prependStateRecord(value: StateRecord): Unit = TODO()
    override val size: Int get() = TODO()
    override val keys: MutableSet<K> get() = TODO()
    override val values: MutableCollection<V> get() = TODO()
    override val entries: MutableSet<MutableMap.MutableEntry<K, V>> get() = TODO()
    override fun isEmpty(): Boolean = TODO()
    override fun containsKey(key: K): Boolean = TODO()
    override fun containsValue(value: V): Boolean = TODO()
    override fun get(key: K): V? = TODO()
    override fun clear(): Unit = TODO()
    override fun remove(key: K): V? = TODO()
    override fun putAll(from: Map<out K, V>): Unit = TODO()
    override fun put(key: K, value: V): V? = TODO()
}
