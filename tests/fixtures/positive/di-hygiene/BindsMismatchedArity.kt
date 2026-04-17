package dihygiene

annotation class Module
annotation class Binds
annotation class Named(val value: String)

interface Foo

class FooImpl : Foo
class BarImpl : Foo

@Module
abstract class FooModule {
    @Binds
    abstract fun bindFoo(@Named("a") a: FooImpl, b: BarImpl): Foo
}
