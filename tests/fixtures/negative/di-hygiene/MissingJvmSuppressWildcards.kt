package dihygiene

annotation class Provides

interface Plugin

class Registry

class PluginsModule {
    @Provides
    fun provideRegistry(plugins: Set<@JvmSuppressWildcards Plugin>): Registry = Registry()

    @Provides
    fun provideMapRegistry(plugins: Map<String, @JvmSuppressWildcards Plugin>): Registry = Registry()

    // Return types do not get wildcards, so they are not flagged.
    @Provides
    fun providePlugins(): Set<Plugin> = emptySet()

    @Provides
    fun providePluginMap(): Map<String, Plugin> = emptyMap()

    // Single-element bindings are fine.
    @Provides
    fun providePlugin(): Plugin = object : Plugin {}

    // List<T> is not a Dagger multibinding; not flagged.
    @Provides
    fun provideListRegistry(plugins: List<Plugin>): Registry = Registry()

    // Not @Provides; not flagged.
    fun helper(plugins: Set<Plugin>): Registry = Registry()
}
