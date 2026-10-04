// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 30
// Go misses the findings below: the declared types are import aliases of
// collection wrappers, and Go reads only the written name (`PluginList`,
// `Registry`), which is not in its wrapper list. Each function is a real
// Dagger @IntoSet binding that returns a collection wrapper, so the message is
// true of it.
package test

import dagger.Provides
import dagger.multibindings.IntoSet
import kotlin.collections.List as PluginList
import java.util.LinkedHashMap as Registry
import java.util.TreeSet as Set

interface Plugin

class PluginModule {
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun importAlias(): PluginList<Plugin> = emptyList()

    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun javaImportAlias(): Registry<String, Plugin> = Registry()

    // Go reports this one: the written name is `Set`. The alias is for
    // TreeSet, a collection outside Go's list but still a collection, so the
    // message is true and FIR keeps it.
    <!IntoSetOnNonSetReturn!>@Provides<!>
    @IntoSet
    fun aliasedTreeSet(): Set<Plugin> = Set()
}
