// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The binding annotation is Dagger's @Provides or @Binds. A Dagger @IntoMap
// function whose only other annotation is a project one named like a binding
// (the first two functions), or holds "@Provides" only in a string argument
// (provideNamed), is not a map contribution (Dagger reports a different error
// for a multibinding annotation on a non-binding method), so the message
// ("Dagger requires a key annotation on every map contribution") is not true
// of it. Neither Go nor FIR reports them.
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
    @ProvidesHandler
    @IntoMap
    fun provideProjectAnnotation(): Handler = HandlerImpl()

    @Provides2
    @IntoMap
    fun provideProjectAnnotation2(): Handler = HandlerImpl()

    @Named("@Provides")
    @IntoMap
    fun provideNamed(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Contribution<!>
    @ContributeToMap
    fun provideTypeAliased(): Handler = HandlerImpl()
}
