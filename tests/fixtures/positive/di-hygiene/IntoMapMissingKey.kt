package dihygiene

annotation class Provides
annotation class IntoMap

interface Handler
class HandlerImpl : Handler

class HandlerModule {
    @Provides
    @IntoMap
    fun provideHandler(): Handler = HandlerImpl()
}
