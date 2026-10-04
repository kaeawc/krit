// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 28
// Divergence: Go reports the functions below because it looks for the text
// `@IntoSet` and `@Provides` / `@Binds` anywhere in the modifier list.
// - `@IntoSet` appears only inside a string argument, so the function is not
//   a multibinding and nothing is contributed to a set.
// - `@BindsOptionalOf` contains the text `@Binds`, but it is not a binding
//   Dagger contributes to a set (Dagger rejects it with @IntoSet).
// Neither contributes a collection to a multibound set, so the message ("the
// contribution will be a Set<List> entry") is not true of them. No finding
// here.
package test

import dagger.BindsOptionalOf
import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoSet
import javax.inject.Named

interface Plugin

@Module
abstract class PluginModule {
    @Provides
    @Named("@IntoSet")
    fun namedList(): List<Plugin> = emptyList()

    @BindsOptionalOf
    @IntoSet
    abstract fun optionalList(): List<Plugin>
}
