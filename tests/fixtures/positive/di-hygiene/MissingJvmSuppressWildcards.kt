package dihygiene

annotation class Provides

interface Plugin

class Registry

class PluginsModule {
    @Provides
    fun provideRegistry(plugins: Set<Plugin>): Registry = Registry()

    @Provides
    fun provideMapRegistry(plugins: Map<String, Plugin>): Registry = Registry()
}
