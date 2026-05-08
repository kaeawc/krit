package dihygiene

annotation class Singleton
annotation class Inject

// Generic but unscoped: fine.
class Cache<K, V> @Inject constructor() {
    fun get(key: K): V? = null
}

// Scoped but non-generic: fine.
@Singleton
class UserRepository @Inject constructor()
