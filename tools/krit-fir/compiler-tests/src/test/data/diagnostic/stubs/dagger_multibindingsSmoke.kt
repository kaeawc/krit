// Smoke: set/map multibinding contributions and declarations.
package stubs

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.multibindings.ClassKey
import dagger.multibindings.ElementsIntoSet
import dagger.multibindings.IntKey
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.Multibinds
import dagger.multibindings.StringKey

interface Plugin

class LoggingPlugin : Plugin

@Module
abstract class PluginModule {
    @Multibinds
    abstract fun plugins(): Set<Plugin>

    @Binds
    @IntoMap
    @ClassKey(LoggingPlugin::class)
    abstract fun bindByClass(plugin: LoggingPlugin): Plugin

    companion object {
        @Provides
        @IntoSet
        fun provideLogging(): Plugin = LoggingPlugin()

        @Provides
        @ElementsIntoSet
        fun provideDefaults(): Set<Plugin> = emptySet()

        @Provides
        @IntoMap
        @StringKey("logging")
        fun provideByName(): Plugin = LoggingPlugin()

        @Provides
        @IntoMap
        @IntKey(1)
        fun provideByNumber(): Plugin = LoggingPlugin()
    }
}
