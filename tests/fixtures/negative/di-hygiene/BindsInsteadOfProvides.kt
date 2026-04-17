package dihygiene

annotation class Module
annotation class Provides
annotation class Binds

interface Foo

class FooImpl : Foo

@Module
abstract class FooModule {
    @Binds
    abstract fun bindFoo(impl: FooImpl): Foo
}
