package dihygiene

annotation class Provides
annotation class IntoSet

interface Plugin

class PluginModule {
    @Provides
    @IntoSet
    fun providePluginList(): List<Plugin> = emptyList()
}
