package dihygiene

import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin

class PluginModule {
    @Provides
    @IntoSet
    fun providePluginList(): List<Plugin> = emptyList()
}
