// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: functions Go and FIR both leave alone.
package test

import dagger.Binds
import dagger.BindsInstance
import dagger.Module
import dagger.Provides
import dagger.multibindings.ElementsIntoSet
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.StringKey
import java.util.LinkedList
import java.util.TreeSet

interface Plugin
class PluginImpl : Plugin

@Module
class PluginModule {
    // A single element is what @IntoSet expects.
    @Provides
    @IntoSet
    fun providePlugin(): Plugin = PluginImpl()

    // No @IntoSet: returning a List is fine.
    @Provides
    fun providePluginList(): List<Plugin> = emptyList()

    // @ElementsIntoSet is the way to contribute a collection.
    @Provides
    @ElementsIntoSet
    fun provideElements(): Set<Plugin> = emptySet()

    // @IntoMap is a different multibinding.
    @Provides
    @IntoMap
    @StringKey("plugins")
    fun provideMapEntry(): List<Plugin> = emptyList()

    // @IntoSet without @Provides or @Binds is not a binding.
    @IntoSet
    fun notABinding(): List<Plugin> = emptyList()

    // Go reads only a declared return type; an inferred one is not checked.
    @Provides
    @IntoSet
    fun inferred() = listOf<Plugin>()

    // Types outside Go's wrapper list, collections or not.
    @Provides
    @IntoSet
    fun linkedList(): LinkedList<Plugin> = LinkedList()

    @Provides
    @IntoSet
    fun treeSet(): TreeSet<String> = TreeSet()

    @Provides
    @IntoSet
    fun sequence(): Sequence<Plugin> = emptySequence()

    @Provides
    @IntoSet
    fun intArray(): IntArray = IntArray(0)

    @Provides
    @IntoSet
    fun entry(): Map.Entry<String, Plugin> = mapOf("a" to PluginImpl() as Plugin).entries.first()

    @Provides
    @IntoSet
    fun string(): String = "plugin"

    // A function type is not a collection, even when it returns one.
    @Provides
    @IntoSet
    fun factory(): () -> List<Plugin> = { emptyList() }

    // A lambda returning a collection inside a provider is not a provider.
    @Provides
    @IntoSet
    fun lambdaHost(): Plugin {
        val make = fun(): List<Plugin> = emptyList()
        return make().firstOrNull() ?: PluginImpl()
    }

    // A local function inside a provider is not itself annotated.
    @Provides
    @IntoSet
    fun localHost(): Plugin {
        fun helper(): List<Plugin> = emptyList()
        return helper().firstOrNull() ?: PluginImpl()
    }

    // Property accessors are not function declarations, so neither Go nor
    // FIR checks them.
    @get:Provides
    @get:IntoSet
    val property: List<Plugin>
        get() = emptyList()
}

@Module
abstract class BindsModule {
    @Binds
    @IntoSet
    abstract fun bindPlugin(impl: PluginImpl): Plugin

    // The collection is a parameter, not the return type.
    @Binds
    @IntoSet
    abstract fun bindFromList(impl: ArrayList<Plugin>): Plugin

    @BindsInstance
    fun instance(plugins: List<Plugin>) {}
}
