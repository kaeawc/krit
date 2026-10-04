// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Map contributions with a key, and functions that are not map contributions.
// Neither Go nor FIR reports any of them.
package test

import dagger.Binds
import dagger.MapKey
import dagger.Module
import dagger.Provides
import dagger.multibindings.ClassKey
import dagger.multibindings.IntKey
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.LongKey
import dagger.multibindings.StringKey
import javax.inject.Qualifier

interface Handler

class HandlerImpl : Handler

@MapKey
annotation class HandlerKey(val value: String)

// Not a map key: a qualifier whose name ends in Key. Go counts any annotation
// named *Key as the key, and FIR keeps that name test (a judgment call that
// matches Go), so the function using it below is not reported.
@Qualifier
annotation class ApiKey

@Module
abstract class KeyedModule {
    @Binds
    @IntoMap
    @ClassKey(HandlerImpl::class)
    abstract fun bindByClass(impl: HandlerImpl): Handler

    companion object {
        @Provides
        @IntoMap
        @StringKey("name")
        fun provideByName(): Handler = HandlerImpl()

        @Provides @IntoMap @IntKey(1) fun provideByInt(): Handler = HandlerImpl()

        @Provides
        @IntoMap
        @LongKey(2L)
        fun provideByLong(): Handler = HandlerImpl()

        @Provides
        @IntoMap
        @HandlerKey("custom")
        fun provideByCustomKey(): Handler = HandlerImpl()

        @Provides
        @IntoMap
        @dagger.multibindings.StringKey("qualified")
        fun provideByQualifiedKey(): Handler = HandlerImpl()

        @Provides
        @IntoMap
        @ApiKey
        fun provideByQualifierNamedKey(): Handler = HandlerImpl()

        // Not @IntoMap.
        @Provides
        fun providePlain(): Handler = HandlerImpl()

        @Provides
        @IntoSet
        fun provideIntoSet(): Handler = HandlerImpl()
    }
}

// @IntoMap without @Provides or @Binds is not a binding, in Go or here.
@IntoMap
fun notABinding(): Handler = HandlerImpl()

class Plain {
    fun provideHandler(): Handler = HandlerImpl()
}
