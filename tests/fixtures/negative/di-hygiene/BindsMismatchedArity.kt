package dihygiene

annotation class Module
annotation class Binds
annotation class Named(val value: String)
annotation class SomeQualifier

interface Foo

class FooImpl : Foo
class QualifiedFooImpl : Foo

@Module
abstract class FooModule {
    @Binds
    abstract fun bindFoo(impl: FooImpl): Foo

    @Binds
    abstract fun bindNamedFoo(@Named("foo") impl: FooImpl): Foo

    @Binds
    @SomeQualifier
    abstract fun bindQualifiedFoo(
        @SomeQualifier
        impl: QualifiedFooImpl
    ): Foo

    @Binds
    abstract fun bindQualifiedList(impls: List<@JvmSuppressWildcards FooImpl>): List<Foo>
}
