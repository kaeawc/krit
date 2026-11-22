package dihygiene

import dagger.Module
import dagger.Provides

interface Foo

class FooImpl : Foo

@Module
abstract class FooModule {
    @Provides
    fun provideFoo(impl: FooImpl): Foo = impl
}
