import dagger.Binds
import dagger.Module

interface Foo

@Module
abstract class ExampleModule {
    @Binds
    abstract fun bind(foo: Foo): Foo
}
