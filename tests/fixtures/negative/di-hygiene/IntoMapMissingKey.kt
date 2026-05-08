package dihygiene

annotation class Provides
annotation class IntoMap
annotation class StringKey(val value: String)
annotation class ClassKey(val value: kotlin.reflect.KClass<*>)

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
