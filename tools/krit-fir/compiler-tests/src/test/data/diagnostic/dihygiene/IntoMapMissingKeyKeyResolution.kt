// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 29
// Divergence (precision): Go decides whether a function has a map key from
// the written annotation name (it must end in "Key"), so it reports both
// functions below. Each has a map key: StringKey under an import alias, and a
// project @MapKey annotation whose name does not end in Key. The message
// ("missing a @*Key annotation; Dagger requires a key annotation") is not
// true of them, and Dagger accepts them. No finding here.
package test

import dagger.MapKey
import dagger.Provides
import dagger.multibindings.IntoMap
import dagger.multibindings.StringKey as SK

interface Handler

class HandlerImpl : Handler

@MapKey
annotation class HandlerType(val value: String)

class KeyResolutionModule {
    @Provides
    @IntoMap
    @SK("aliased")
    fun provideAliasedKey(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @HandlerType("custom")
    fun provideCustomMapKey(): Handler = HandlerImpl()
}
