// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.collections.immutable

interface ImmutableCollection<out E> : Collection<E>

interface ImmutableList<out E> : List<E>, ImmutableCollection<E> {
    override fun subList(fromIndex: Int, toIndex: Int): ImmutableList<E> = TODO()
}

interface ImmutableSet<out E> : Set<E>, ImmutableCollection<E>

interface ImmutableMap<K, out V> : Map<K, V> {
    override val keys: ImmutableSet<K>

    override val values: ImmutableCollection<V>

    override val entries: ImmutableSet<Map.Entry<K, V>>
}

interface PersistentList<out E> : ImmutableList<E> {
    fun add(element: @UnsafeVariance E): PersistentList<E>

    fun addAll(elements: Collection<@UnsafeVariance E>): PersistentList<E>

    fun remove(element: @UnsafeVariance E): PersistentList<E>

    fun removeAt(index: Int): PersistentList<E>

    fun set(index: Int, element: @UnsafeVariance E): PersistentList<E>
}

interface PersistentSet<out E> : ImmutableSet<E> {
    fun add(element: @UnsafeVariance E): PersistentSet<E>

    fun remove(element: @UnsafeVariance E): PersistentSet<E>
}

interface PersistentMap<K, out V> : ImmutableMap<K, V> {
    fun put(key: K, value: @UnsafeVariance V): PersistentMap<K, V>

    fun remove(key: K): PersistentMap<K, V>
}

fun <E> persistentListOf(vararg elements: E): PersistentList<E> = TODO()

fun <E> persistentListOf(): PersistentList<E> = TODO()

fun <E> persistentSetOf(vararg elements: E): PersistentSet<E> = TODO()

fun <K, V> persistentMapOf(vararg pairs: Pair<K, V>): PersistentMap<K, V> = TODO()

fun <T> Iterable<T>.toImmutableList(): ImmutableList<T> = TODO()

fun <T> Iterable<T>.toPersistentList(): PersistentList<T> = TODO()

fun <T> Iterable<T>.toImmutableSet(): ImmutableSet<T> = TODO()

fun <T> Iterable<T>.toPersistentSet(): PersistentSet<T> = TODO()

fun <K, V> Map<K, V>.toImmutableMap(): ImmutableMap<K, V> = TODO()

fun <K, V> Map<K, V>.toPersistentMap(): PersistentMap<K, V> = TODO()
