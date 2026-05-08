package dihygiene

annotation class Module
annotation class Binds
annotation class Provides

interface A
class AImpl : A
interface B
class BImpl : B

@Module
abstract class GoodModule {
    @Binds
    abstract fun bindA(impl: AImpl): A

    companion object {
        @Provides
        fun provideB(): B = BImpl()
    }
}

@Module
object PlainProviderModule {
    @Provides
    fun provideB(): B = BImpl()
}

@Module
abstract class BindsOnlyModule {
    @Binds
    abstract fun bindA(impl: AImpl): A
}
