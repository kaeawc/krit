// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The annotations below are local annotation classes, declared in this file
// with no DI framework's imported, not a DI framework's multibinding
// annotations, so no framework collects the return value into a set and the
// message ("Dagger collects by return type, so the contribution will be a
// Set<List> entry") is not true of this code. Neither Go nor FIR reports them.
package test

annotation class Provides
annotation class Binds
annotation class IntoSet

interface Plugin

class PluginModule {
    @Provides
    @IntoSet
    fun providePluginList(): List<Plugin> = emptyList()

    @Binds
    @IntoSet
    fun bindSet(impl: HashSet<Plugin>): Set<Plugin> = impl
}

@Provides
@IntoSet
fun topLevel(): Map<String, Plugin> = emptyMap()
