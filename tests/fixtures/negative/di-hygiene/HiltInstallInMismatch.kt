package dihygiene

annotation class Module
annotation class Provides
annotation class InstallIn(val value: kotlin.reflect.KClass<*>)
annotation class ActivityScoped
annotation class Singleton

class SingletonComponent
class ActivityComponent

interface Foo
class FooImpl : Foo
interface Bar
class BarImpl : Bar

@Module
@InstallIn(ActivityComponent::class)
class ActivityFooModule {
    @Provides
    @ActivityScoped
    fun foo(): Foo = FooImpl()
}

@Module
@InstallIn(SingletonComponent::class)
class SingletonFooModule {
    @Provides
    @Singleton
    fun bar(): Bar = BarImpl()

    // Unscoped @Provides; not flagged.
    @Provides
    fun foo(): Foo = FooImpl()
}
