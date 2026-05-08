package dihygiene

annotation class Provides

interface Plugin

class PluginsModule {
    @Provides
    fun providePlugins(): Set<Plugin> = emptySet()

    @Provides
    fun providePluginMap(): Map<String, Plugin> = emptyMap()
}
