// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence: Go reads only a declared return type, so it misses every
// finding below. Each provider's inferred return type is a collection
// wrapper, which Dagger collects as one Set element, so the message is true
// of it.
package test

import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet

interface Plugin
class PluginImpl : Plugin

@Module
class PluginModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun list() = listOf<Plugin>(PluginImpl())

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun mutableSet() = mutableSetOf<Plugin>()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun arrayList() = ArrayList<Plugin>()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun array() = arrayOf<Plugin>(PluginImpl())

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun map() = mapOf("a" to PluginImpl())

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun nullable() = if (System.nanoTime() > 0) listOf<Plugin>() else null

    // A single element is what @IntoSet expects.
    @Provides
    @IntoSet
    fun single() = PluginImpl()

    // Not in Go's wrapper list: a Sequence is not a collection.
    @Provides
    @IntoSet
    fun sequence() = sequenceOf<Plugin>()

    // A function type is not a collection, even when it returns one.
    @Provides
    @IntoSet
    fun factory() = { listOf<Plugin>() }

    // A block body without a declared type returns Unit.
    @Provides
    @IntoSet
    fun unit() {
        listOf<Plugin>()
    }
}
