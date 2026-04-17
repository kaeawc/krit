package digraph

annotation class Module
annotation class Provides
annotation class Component
annotation class Inject

interface Foo

class FooImpl : Foo

@Module
object AppModule {
    @Provides
    fun provideFoo(): Foo = FooImpl()
}

class Dashboard @Inject constructor(val foo: Foo)

@Component
interface AppComponent {
    fun dashboard(): Dashboard
}
