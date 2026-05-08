package dihygiene

annotation class Module
annotation class Provides
annotation class IntoMap
annotation class StringKey(val value: String)

interface Handler
class HandlerA : Handler
class HandlerB : Handler

@Module
object HandlerModule {
    @Provides @IntoMap @StringKey("foo")
    fun provideA(): Handler = HandlerA()

    @Provides @IntoMap @StringKey("bar")
    fun provideB(): Handler = HandlerB()
}

// Same key but different enclosing module — not flagged.
@Module
object OtherModule {
    @Provides @IntoMap @StringKey("foo")
    fun provideC(): Handler = HandlerA()
}
