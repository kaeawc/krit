// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 22, 27
// Divergence: Go reports every function below because it matches the
// annotation text `@IntoSet` and `@Provides` / `@Binds`. These are local
// annotation classes, not a DI framework's multibinding annotations, so no
// framework collects the return value into a set and the message ("Dagger
// collects by return type, so the contribution will be a Set<List> entry") is
// not true of this code. No finding here.
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
