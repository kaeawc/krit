// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 32
// Go's name tests read the annotation name as written, so an import alias
// counts by its own name: an alias written @...Key is a map key, and one
// written @Provides... or @Binds... is a binding annotation. FIR applies the
// same name tests to the written name (matching Go), as it does to a project
// annotation named @ProvidesHandler or a qualifier named @ApiKey.
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

    // The alias is written ProvidesCustom, so Go's binding test counts it:
    // a Dagger @IntoMap function without a key, reported by both.
    <!IntoMapMissingKey!>@ProvidesCustom<!>
    @IntoMap
    fun provideAliasNamedProvides(): Handler = HandlerImpl()
}
