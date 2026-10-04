package dihygiene

import dagger.Provides
import dagger.multibindings.ClassKey
import dagger.multibindings.IntoMap
import dagger.multibindings.StringKey

interface Handler
class HandlerImpl : Handler
class OtherHandler : Handler

class HandlerModule {
    @Provides
    @IntoMap
    @StringKey("foo")
    fun provideHandler(): Handler = HandlerImpl()

    @Provides
    @IntoMap
    @ClassKey(OtherHandler::class)
    fun provideOther(): Handler = OtherHandler()

    // Not @IntoMap; should not fire.
    @Provides
    fun providePlain(): Handler = HandlerImpl()
}
