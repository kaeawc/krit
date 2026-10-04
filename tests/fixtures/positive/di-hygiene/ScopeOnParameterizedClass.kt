package dihygiene

import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}
