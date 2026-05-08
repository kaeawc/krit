package dihygiene

annotation class Singleton
annotation class Inject

@Singleton
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}
