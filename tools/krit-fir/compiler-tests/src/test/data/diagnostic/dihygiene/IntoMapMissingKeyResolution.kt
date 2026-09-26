// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): Go matches the written annotation text "@IntoMap",
// "@Provides", and "@Binds", so it misses Dagger's annotations when they are
// fully qualified, import-aliased, or written in the bracketed @[...] form
// (a group of several annotations or a single one: @[IntoMap], @[Provides],
// @[Binds]). Each function below is still a Dagger map contribution without a
// key, so FIR reports it.
package test

import dagger.Binds
import dagger.Provides
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoMap as MapContribution

interface Handler

class HandlerImpl : Handler

abstract class ResolutionModule {
    <!IntoMapMissingKey!>@Provides<!>
    @dagger.multibindings.IntoMap
    fun provideQualifiedIntoMap(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@dagger.Provides<!>
    @IntoMap
    fun provideQualifiedProvides(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Provides<!>
    @MapContribution
    fun provideAliased(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@[Provides IntoMap]<!>
    fun provideBracketed(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@[Binds]<!> @IntoMap
    abstract fun bindBracketed(impl: HandlerImpl): Handler

    <!IntoMapMissingKey!>@Provides<!>
    @[IntoMap]
    fun provideBracketedIntoMap(): Handler = HandlerImpl()

    companion object Named {
        <!IntoMapMissingKey!>@[Provides]<!> @IntoMap @JvmStatic fun provideInNamedCompanion(): Handler = HandlerImpl()
    }
}
