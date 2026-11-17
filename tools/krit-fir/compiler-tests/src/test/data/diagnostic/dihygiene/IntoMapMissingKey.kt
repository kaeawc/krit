// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 26, 29, 33, 37, 41, 48, 53, 61, 68, 74, 92, 98
// Dagger map contributions without a map key, the shapes Go reports: a
// function with @IntoMap and @Provides/@Binds, wherever it is declared. The
// finding sits on the declaration's first line (its first annotation), like
// Go, and names the function as written.
package test

import dagger.Binds
import dagger.BindsInstance
import dagger.BindsOptionalOf
import dagger.Module
import dagger.Provides
import dagger.multibindings.IntoMap

interface Handler

class HandlerImpl : Handler

@Module
class HandlerModule {
    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun provideHandler(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@IntoMap<!> @Provides fun provideReversed(): Handler = HandlerImpl()

    /** KDoc is not part of the declaration's first line. */
    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun provideDocumented(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun `provide with spaces`(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun String.provideExtension(): Handler = HandlerImpl()

    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    suspend fun provideSuspend(): Handler = HandlerImpl()
}

@Module
abstract class BindsModule {
    <!IntoMapMissingKey!>@Binds<!>
    @IntoMap
    abstract fun bindHandler(impl: HandlerImpl): Handler

    companion object {
        <!IntoMapMissingKey!>@Provides<!>
        @IntoMap
        fun provideFromCompanion(): Handler = HandlerImpl()
    }
}

@Module
interface InterfaceModule {
    <!IntoMapMissingKey!>@Binds<!>
    @IntoMap
    fun bindHandler(impl: HandlerImpl): Handler
}

@Module
object ObjectModule {
    <!IntoMapMissingKey!>@JvmStatic<!>
    @Provides
    @IntoMap
    fun provideFromObject(): Handler = HandlerImpl()
}

<!IntoMapMissingKey!>@Provides<!>
@IntoMap
fun provideTopLevel(): Handler = HandlerImpl()

// @BindsOptionalOf and @BindsInstance declare no binding @IntoMap contributes
// to (Dagger rejects @IntoMap there for another reason), so neither reports.
@Module
abstract class OddBindingsModule {
    @BindsOptionalOf
    @IntoMap
    abstract fun optionalHandler(): Handler

    @BindsInstance
    @IntoMap
    abstract fun instanceHandler(): Handler
}

val anonymousModule = object {
    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun provideFromAnonymous(): Handler = HandlerImpl()
}

fun outer(): Handler {
    <!IntoMapMissingKey!>@Provides<!>
    @IntoMap
    fun provideLocal(): Handler = HandlerImpl()
    return provideLocal()
}
