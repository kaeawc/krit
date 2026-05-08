package dihygiene

annotation class Module
annotation class Binds
annotation class Provides

interface A
class AImpl : A
interface B
class BImpl : B

@Module
abstract class FooModule {
    @Binds
    abstract fun bindA(impl: AImpl): A

    @Provides
    fun provideB(): B = BImpl()
}
