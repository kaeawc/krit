package dihygiene

import javax.inject.Inject
import javax.inject.Singleton

// Generic but unscoped: fine.
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}

// Scoped but non-generic: fine.
@Singleton
class UserRepository @Inject constructor()

// Dagger keys scoped bindings by their full type.
@Singleton
class ScopedCache<T> @Inject constructor()
