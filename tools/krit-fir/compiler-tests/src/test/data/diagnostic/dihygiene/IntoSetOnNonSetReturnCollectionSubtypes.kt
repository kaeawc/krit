// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 39, 43, 47, 51, 55, 59, 67, 71, 81, 86
// Positive: the return type is a collection that is not one of the stdlib
// classes, but its name is in Go's wrapper list. Go reports any type whose
// last dotted segment is in the list, from whatever package: a project's own
// collection, a local class, or a third-party one shaped like Eclipse
// Collections' `MutableList` or Vavr's `HashMap` (their Java declarations, in
// their real packages, are in IntoSetOnNonSetReturnFrameworksTest). Each one
// is a collection, so the message is true of it and FIR keeps the finding.
package test

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin

// A project collection named like the stdlib one.
class Set<T>(private val items: kotlin.collections.Set<T>) : kotlin.collections.Set<T> by items

// A third-party list shaped like Eclipse Collections': a MutableList subtype.
interface MutableList<T> : kotlin.collections.MutableList<T>

// A third-party map shaped like Vavr's: an Iterable of pairs through its own
// interface, not a kotlin.collections.Map.
interface Traversable<T> : Iterable<T>
interface HashMap<K, V> : Traversable<Pair<K, V>>

class Registry {
    class Map<K, V>(entries: kotlin.collections.Map<K, V>) : kotlin.collections.Map<K, V> by entries
}

// A type alias named like a wrapper, for a collection outside Go's list.
typealias Collection<T> = java.util.LinkedList<T>

@Module
abstract class PluginModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun customSet(plugins: kotlin.collections.Set<Plugin>): Set<Plugin> = Set(plugins)

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun nullableCustomSet(): Set<Plugin>? = null

    <!IntoSetOnNonSetReturn!>@Binds<!>
    @IntoSet
    abstract fun eclipse(impl: MutableList<Plugin>): MutableList<Plugin>

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun vavr(map: HashMap<String, Plugin>): HashMap<String, Plugin> = map

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun nested(): Registry.Map<String, Plugin> = Registry.Map(emptyMap())

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun aliasOfLinkedList(): Collection<Plugin> = java.util.LinkedList()

    // A type parameter named like a wrapper and bounded by a collection
    // returns a collection. Like the generic positive in
    // IntoSetOnNonSetReturn.kt, the rule does not model whether the DI
    // processor accepts a generic provider, so this matches Go.
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun <List : kotlin.collections.List<Plugin>> bounded(value: List): List = value

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun <L : kotlin.collections.List<Plugin>, Iterable : L> boundedThroughParameter(value: Iterable): Iterable = value
}

fun host(plugins: kotlin.collections.List<Plugin>): Int {
    // A local collection class: its class id is local, so the checker reads
    // its supertypes through the type's own lookup tag.
    class List<T>(items: kotlin.collections.List<T>) : kotlin.collections.List<T> by items

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun local(): List<Plugin> = List(plugins)

    val anonymous = object {
        <!IntoSetOnNonSetReturn!>@Provides<!>
        @IntoSet
        fun member(): List<Plugin> = List(plugins)
    }
    return local().size + anonymous.member().size
}
