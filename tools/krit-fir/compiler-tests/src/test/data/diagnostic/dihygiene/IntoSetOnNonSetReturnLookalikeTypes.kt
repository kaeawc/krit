// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30, 34, 38, 42, 46, 50
// Divergence: Go reports every function below because the last dotted segment
// of the declared type's name is in its wrapper list (`List`, `Set`,
// `Collection`, `Array`). None of these types is a collection: a class that
// happens to be named List, a type alias named Collection for a plain class,
// a nested class named Set, and type parameters named Set and Array. The
// message ("returns ..., a collection wrapper") is not true of them, so there
// is no finding here.
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
