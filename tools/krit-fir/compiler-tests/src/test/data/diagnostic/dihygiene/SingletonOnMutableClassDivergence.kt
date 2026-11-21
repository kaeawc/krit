// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 78, 85, 92, 99, 108
package test

import javax.inject.Singleton
import javax.inject.Singleton as Shared
import kotlin.collections.mutableListOf as newEntries
import kotlin.jvm.Volatile as Visible
import kotlin.jvm.Synchronized as Locked
import androidx.annotation.MainThread as OnMain

typealias SharedScope = Singleton
typealias Entries = MutableList<String>
class EntryList : ArrayList<String>()
fun entries(): Entries = kotlin.collections.mutableListOf()

// Go misses an aliased scope; FIR resolves javax.inject.Singleton.
@Shared
class ImportAlias {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

// Go misses a type-aliased scope; FIR resolves javax.inject.Singleton.
@SharedScope
class ScopeAlias {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

// Go misses the aliased factory; FIR resolves the known Kotlin mutable factory.
@Singleton
class FactoryAlias {
    <!SingletonOnMutableClass!>val values = newEntries<String>()<!>
}

// Both are silent: a MutableList return type alone does not prove unsynchronized state.
@Singleton
class TypeAlias {
    val values = entries()
}

// Both are silent: a project subtype may add synchronization.
@Singleton
class Subtype {
    val values = EntryList()
}

fun nullableEntries(): MutableList<String>? = kotlin.collections.mutableListOf()

// Both are silent: an unknown nullable factory may return a thread-safe collection.
@Singleton
class NullableCollection {
    val values = nullableEntries()
}

@javax.inject.Qualifier
annotation class MainDispatcher

// Both exempt the realistic qualified dispatcher constructor parameter.
@Singleton
class NamedDispatcher(@MainDispatcher private val mainDispatcher: kotlinx.coroutines.CoroutineDispatcher) {
    var value = 0
    fun bump() { value++ }
}

// Go's text marker skips this var because its initializer contains "Atomic".
@Singleton
class MarkerInLiteral {
    <!SingletonOnMutableClass!>var value = "Atomic"<!>
    fun reset() { value = "next" }
}

// Go mistakes references to the shadowing local for accesses to the member.
// FIR resolves the local symbol and exempts the never-assigned lateinit member.
@Singleton
class ShadowedAssignment {
    lateinit var value: String
    fun unrelated() { var value = ""; value = "next"; println(value) }
}

// Go misses the aliased protection annotation and reports the var.
@Singleton
class AliasedProtection {
    @Visible var value = 0
    fun bump() { value++ }
}

// Go misses the aliased synchronized annotation and reports the var.
@Singleton
class AliasedLock {
    private var value = 0
    @Locked fun bump() { value++ }
}

// Go misses the aliased thread annotation and reports the var.
@Singleton
class AliasedThread {
    private var value = 0
    @OnMain fun bump() { value++ }
}

typealias Dispatcher = kotlinx.coroutines.MainCoroutineDispatcher

// Go misses a type alias of the confinement type and reports the var.
@Singleton
class AliasedDispatcher(private val dispatcher: Dispatcher) {
    var value = 0
    fun bump() { value++ }
}
