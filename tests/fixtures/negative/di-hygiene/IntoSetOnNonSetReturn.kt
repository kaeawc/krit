package dihygiene

import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin
class PluginImpl : Plugin

class PluginModule {
    @Provides
    @IntoSet
    fun providePlugin(): Plugin = PluginImpl()

    // No @IntoSet - returning a List is fine.
    @Provides
    fun providePluginList(): List<Plugin> = emptyList()
}
