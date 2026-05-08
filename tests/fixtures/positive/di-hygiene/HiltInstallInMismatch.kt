package dihygiene

annotation class Module
annotation class Provides
annotation class InstallIn(val value: kotlin.reflect.KClass<*>)
annotation class ActivityScoped

class SingletonComponent

interface Foo
class FooImpl : Foo

@Module
@InstallIn(SingletonComponent::class)
class FooModule {
    @Provides
    @ActivityScoped
    fun foo(): Foo = FooImpl()
}
