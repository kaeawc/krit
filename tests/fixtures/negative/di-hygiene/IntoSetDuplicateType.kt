package dihygiene

annotation class Module
annotation class Provides
annotation class IntoSet

interface Plugin
class PluginA : Plugin
class PluginB : Plugin

@Module
object PluginModule {
    @Provides @IntoSet
    fun provideA(): Plugin = PluginA()

    @Provides @IntoSet
    fun provideB(): Plugin = PluginB()
}
