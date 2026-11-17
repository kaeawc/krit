// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// None of the functions below carries Dagger's @IntoMap: the first uses a
// project annotation named IntoMap (declared in this file, Dagger's not
// imported), the second one whose name only starts with IntoMap, and the
// third has "@IntoMap" only inside a string argument. None is a Dagger map
// contribution, so the message ("@IntoMap function ... Dagger requires a key
// annotation on every map contribution") is not true of them. Neither Go nor
// FIR reports them.
package test

import dagger.Provides
import javax.inject.Named

annotation class IntoMap

annotation class IntoMapping

interface Handler

class HandlerImpl : Handler

class LookalikeModule {
    @Provides
    @IntoMap
    fun provideLookalike(): Handler = HandlerImpl()

    @Provides
    @IntoMapping
    fun providePrefixed(): Handler = HandlerImpl()

    @Provides
    @Named("@IntoMap")
    fun provideNamed(): Handler = HandlerImpl()
}
