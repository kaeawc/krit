// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go's key test reads the annotation name as written, so an import alias
// counts by its own name: an alias written @...Key is a map key. FIR applies
// the same key test to the written name (matching Go), as it does to a
// qualifier named @ApiKey. A binding annotation is Dagger's @Provides or
// @Binds, so an alias of another class written @ProvidesCustom is not one.
package test

import dagger.Provides
import dagger.multibindings.IntoMap
import test.Custom as ProvidesCustom
import test.Marker as MarkerKey

annotation class Marker

annotation class Custom

interface Handler

class HandlerImpl : Handler

class WrittenNameModule {
    // The alias is written MarkerKey, so Go's key test counts it as the key.
    @Provides
    @IntoMap
    @MarkerKey
    fun provideAliasNamedKey(): Handler = HandlerImpl()

    // The alias written ProvidesCustom names test.Custom, not a binding, so
    // the function is not a map contribution; neither Go nor FIR reports it.
    @ProvidesCustom
    @IntoMap
    fun provideAliasNamedProvides(): Handler = HandlerImpl()
}
