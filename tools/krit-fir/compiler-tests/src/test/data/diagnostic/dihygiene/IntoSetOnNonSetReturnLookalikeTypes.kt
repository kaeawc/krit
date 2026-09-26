// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 31, 35, 39, 43, 47, 51, 60, 75, 79, 83, 87, 91
// Divergence: Go reports every function below because the last dotted segment
// of the declared type's name is in its wrapper list (`List`, `Set`,
// `Collection`, `Array`). None of these types is a collection: a class that
// happens to be named List, a type alias named Collection for a plain class,
// a nested class named Set, type parameters named Set and Array (bounded by a
// non-collection or unbounded), a local class named Set, and function types
// (see the comment at the end). The message ("returns ..., a collection
// wrapper") is not true of them, so there is no finding here.
package test

import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin

class List<T>(val value: T)

class Box(val plugin: Plugin)

typealias Collection = Box

class Holder {
    class Set(val plugin: Plugin)
}

@Module
class PluginModule {
    @Provides
    @IntoSet
    fun localClass(plugin: Plugin): List<Plugin> = List(plugin)

    @Provides
    @IntoSet
    fun nullableLocalClass(plugin: Plugin): List<Plugin>? = List(plugin)

    @Provides
    @IntoSet
    fun aliasNamedCollection(plugin: Plugin): Collection = Box(plugin)

    @Provides
    @IntoSet
    fun nestedNamedSet(plugin: Plugin): Holder.Set = Holder.Set(plugin)

    @Provides
    @IntoSet
    fun <Set : Plugin> typeParameter(value: Set): Set = value

    @Provides
    @IntoSet
    fun <Array> unboundedTypeParameter(value: Array): Array = value
}

fun host(plugin: Plugin): Any {
    // A local class named Set that is not a collection.
    class Set<T>(val value: T)

    @Provides
    @IntoSet
    fun local(): Set<Plugin> = Set(plugin)
    return local()
}

// Divergence: Go cuts a function type's text at its first '<' and keeps the
// last dotted segment, so a function type that mentions a qualified
// collection reads as that collection (even one that is only a lambda
// parameter of a Unit-returning function type). The declared type is a
// function (kotlin.FunctionN), not a collection wrapper, so the message is
// not true of it. No finding here. (`() -> List<Plugin>`, which Go also
// leaves alone, is in IntoSetOnNonSetReturnNegative.kt.)
@Module
class FunctionTypeModule {
    @Provides
    @IntoSet
    fun fnTypeQualified(): () -> kotlin.collections.List<Plugin> = { emptyList() }

    @Provides
    @IntoSet
    fun fnTypeJava(): (Plugin) -> java.util.HashSet<Plugin> = { java.util.HashSet() }

    @Provides
    @IntoSet
    fun fnTypeNullable(): (() -> kotlin.collections.List<Plugin>)? = null

    @Provides
    @IntoSet
    fun suspendFn(): suspend () -> kotlin.collections.Set<Plugin> = { emptySet() }

    @Provides
    @IntoSet
    fun fnTypeGenericArg(): (kotlin.collections.Map<String, Int>) -> Unit = {}
}
