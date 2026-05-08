import dagger.Binds
import dagger.Module

interface Foo

class FooImpl : Foo

@Module
abstract class ExampleModule {
    @Binds
    abstract fun bind(impl: FooImpl): Foo
}
