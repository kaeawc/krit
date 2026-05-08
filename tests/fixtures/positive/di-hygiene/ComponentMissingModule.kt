package fixtures.dihygiene

import kotlin.reflect.KClass

annotation class Component(val modules: Array<KClass<*>> = [])
annotation class Module
annotation class Provides
annotation class Binds
annotation class Inject

interface Foo

class FooImpl @Inject constructor() : Foo

class Feature(val foo: Foo)

@Module
object AModule {
    @Provides
    fun provideFeature(foo: Foo): Feature = Feature(foo)
}

@Module
abstract class BModule {
    @Binds
    abstract fun bindFoo(impl: FooImpl): Foo
}

@Component(modules = [AModule::class])
interface AppComponent {
    fun feature(): Feature
}
