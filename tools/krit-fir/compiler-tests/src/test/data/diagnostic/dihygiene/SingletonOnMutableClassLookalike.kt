// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 20, 24
package test

annotation class Singleton
annotation class Volatile
annotation class MainThread
class ArrayList<T>
class Handler
class AtomicCounter
fun <T> mutableListOf(): List<T> = emptyList()
fun <T> synchronized(lock: Any, action: () -> T): T = action()

// Go reports the same-named annotation; FIR sees no singleton scope.
@Singleton
class ScopeLookalike { var value = 0 }

// Go reports a same-named factory, but the resolved collection is read-only.
@javax.inject.Singleton
class FactoryLookalike { val values = mutableListOf<String>() }

// Go reports a same-named project class that is not a mutable collection.
@javax.inject.Singleton
class TypeLookalike { val values = ArrayList<String>() }

// Go skips a lookalike Volatile annotation; it protects no state.
@javax.inject.Singleton
class ProtectionLookalike {
    <!SingletonOnMutableClass!>@Volatile var value = 0<!>
    fun bump() { value++ }
}

// Go skips a lookalike thread annotation; it confines no state.
@MainThread @javax.inject.Singleton
class ThreadLookalike {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

// Go skips a same-named constructor type; the project class is not a Handler.
@javax.inject.Singleton
class HandlerLookalike(private val handler: Handler) {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

// Go's Atomic text marker skips this var; the project class is not atomic.
@javax.inject.Singleton
class AtomicLookalike {
    <!SingletonOnMutableClass!>var value = AtomicCounter()<!>
    fun reset() { value = AtomicCounter() }
}

// Go considers the same-named call a lock; its resolved body takes no lock.
@javax.inject.Singleton
class LockLookalike {
    <!SingletonOnMutableClass!>private var value = 0<!>
    fun bump() { synchronized(this) { value++ } }
}
