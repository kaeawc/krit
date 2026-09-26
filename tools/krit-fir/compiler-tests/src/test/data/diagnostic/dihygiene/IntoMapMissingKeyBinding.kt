// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 35, 39, 43
// The binding annotation: Go accepts any annotation written @Provides... or
// @Binds..., so a Dagger @IntoMap function with a project annotation named
// that way is reported by both (the first two functions).
//
// Divergence: Go also finds "@Provides" inside a string argument, so it
// reports provideNamed, which has Dagger's @IntoMap but no binding
// annotation. It is not a map contribution (Dagger reports a different error
// for a multibinding annotation on a non-binding method), so the message
// ("Dagger requires a key annotation on every map contribution") is not true
// of it. No finding here.
//
// Divergence (recall): Go misses a type-aliased @IntoMap and @Provides;
// provideTypeAliased is a map contribution without a key, so FIR reports it.
package test

import dagger.Provides
import dagger.multibindings.IntoMap
import javax.inject.Named

annotation class Provides2

annotation class ProvidesHandler

typealias ContributeToMap = IntoMap

typealias Contribution = Provides

interface Handler

class HandlerImpl : Handler

class BindingModule {
    <!IntoMapMissingKey!>@ProvidesHandler<!>
    @IntoMap
    fun provideProjectAnnotation(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Provides2<!>
    @IntoMap
    fun provideProjectAnnotation2(): Handler = HandlerImpl()

    @Named("@Provides")
    @IntoMap
    fun provideNamed(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Contribution<!>
    @ContributeToMap
    fun provideTypeAliased(): Handler = HandlerImpl()
}
