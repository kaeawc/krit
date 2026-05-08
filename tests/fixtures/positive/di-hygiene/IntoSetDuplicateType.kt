package dihygiene

annotation class Module
annotation class Provides
annotation class IntoSet

interface Plugin
class PluginImpl : Plugin

@Module
object PluginModule {
    @Provides @IntoSet
    fun provideA(): Plugin = PluginImpl()

    @Provides @IntoSet
    fun provideB(): Plugin = PluginImpl()
}
