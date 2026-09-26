// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 28, 32
// Divergence: Go matches "@IntoMap", "@Provides", and "@Binds" as substrings
// of the function's annotation text, so it reports every function below. None
// carries Dagger's @IntoMap: the first uses a project annotation named
// IntoMap, the second one whose name only starts with IntoMap, and the third
// has "@IntoMap" only inside a string argument. None is a Dagger map
// contribution, so the message ("@IntoMap function ... Dagger requires a key
// annotation on every map contribution") is not true of them. No finding here.
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
