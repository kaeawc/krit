// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence: Go reads only function declarations, so it misses these
// property bindings. Dagger reads `@get:Provides @get:IntoSet` on the getter,
// which provides a collection wrapper as one Set element, so the message
// ("@IntoSet property ... a collection wrapper") is true of each.
package test

import dagger.Module
import dagger.Provides
import dagger.multibindings.ElementsIntoSet
import dagger.multibindings.IntoSet

interface Plugin
class PluginImpl : Plugin

@Module
object PluginModule {
    <!IntoSetOnNonSetReturn!>@get:Provides<!>
    @get:IntoSet
    val plugins: List<Plugin>
        get() = emptyList()

    // An inferred getter type.
    <!IntoSetOnNonSetReturn!>@get:Provides<!>
    @get:IntoSet
    val pluginMap get() = mapOf("a" to PluginImpl())

    // A single element is what @IntoSet expects.
    @get:Provides
    @get:IntoSet
    val plugin: Plugin
        get() = PluginImpl()

    // @ElementsIntoSet is the way to contribute a collection.
    @get:Provides
    @get:ElementsIntoSet
    val elements: Set<Plugin>
        get() = emptySet()

    // No @IntoSet: a collection binding is fine.
    @get:Provides
    val all: List<Plugin>
        get() = emptyList()
}

fun localHost(): Int {
    // A local property is not a binding.
    val local: List<Plugin> = emptyList()
    return local.size
}
