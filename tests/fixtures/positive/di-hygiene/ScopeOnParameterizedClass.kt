package dihygiene

import javax.inject.Inject
import org.koin.core.annotation.Singleton

@Singleton
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}
