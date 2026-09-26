package dihygiene

import dagger.Provides
import dagger.multibindings.IntoMap

interface Handler
class HandlerImpl : Handler

class HandlerModule {
    @Provides
    @IntoMap
    fun provideHandler(): Handler = HandlerImpl()
}
