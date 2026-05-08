package dihygiene

annotation class Provides

interface Plugin

class PluginsModule {
    @Provides
    fun providePlugins(): Set<@JvmSuppressWildcards Plugin> = emptySet()

    @Provides
    fun providePluginMap(): Map<String, @JvmSuppressWildcards Plugin> = emptyMap()

    // Single-element bindings are fine.
    @Provides
    fun providePlugin(): Plugin = object : Plugin {}

    // List<T> is not a Dagger multibinding; not flagged.
    @Provides
    fun providePluginList(): List<Plugin> = emptyList()

    // Not @Provides; not flagged.
    fun helper(): Set<Plugin> = emptySet()
}
