package dihygiene

annotation class Module
annotation class Provides
annotation class Binds

interface Foo

class FooImpl : Foo

@Module
abstract class FooModule {
    @Provides
    fun provideFoo(impl: FooImpl): Foo = impl
}
