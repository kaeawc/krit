package dihygiene

import dagger.Module
import dagger.Binds

interface Foo

class FooImpl : Foo

@Module
abstract class FooModule {
    @Binds
    abstract fun bindFoo(impl: FooImpl): Foo
}
