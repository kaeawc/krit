// Smoke: Dagger modules, components, subcomponents, scopes, qualifiers, Lazy/Provider.
package stubs

import dagger.Binds
import dagger.BindsInstance
import dagger.BindsOptionalOf
import dagger.Component
import dagger.Lazy
import dagger.MapKey
import dagger.Module
import dagger.Provides
import dagger.Reusable
import dagger.Subcomponent
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Provider
import javax.inject.Qualifier
import javax.inject.Scope
import javax.inject.Singleton

@Qualifier
@Retention(AnnotationRetention.RUNTIME)
annotation class ApiUrl

@Scope
@Retention(AnnotationRetention.RUNTIME)
annotation class ActivityScoped

@MapKey
annotation class ScreenKey(val value: String)

interface DaggerRepo

@Singleton
class DaggerRepoImpl @Inject constructor(
    @ApiUrl private val url: String,
    @Named("timeout") private val timeout: Long,
    private val lazyClock: Lazy<DaggerClock>,
    private val clockProvider: Provider<DaggerClock>,
) : DaggerRepo {
    fun now(): Long = lazyClock.get().now() + clockProvider.get().now() + timeout + url.length
}

@Reusable
class DaggerClock @Inject constructor() {
    fun now(): Long = 0L
}

class FieldInjected {
    @Inject
    lateinit var clock: DaggerClock

    @set:Inject
    var repo: DaggerRepo? = null
}

@Module(includes = [DaggerNetworkModule::class], subcomponents = [DaggerLoginComponent::class])
abstract class DaggerRepoModule {
    @Binds
    abstract fun bindRepo(impl: DaggerRepoImpl): DaggerRepo

    @BindsOptionalOf
    abstract fun optionalClock(): DaggerClock
}

@Module
object DaggerNetworkModule {
    @Provides
    @ApiUrl
    fun provideUrl(): String = "https://example.com"

    @Provides
    @Named("timeout")
    @Singleton
    fun provideTimeout(): Long = 30L
}

@ActivityScoped
@Subcomponent(modules = [DaggerNetworkModule::class])
interface DaggerLoginComponent {
    fun repo(): DaggerRepo

    @Subcomponent.Factory
    interface Factory {
        fun create(): DaggerLoginComponent
    }
}

@Singleton
@Component(modules = [DaggerRepoModule::class], dependencies = [])
interface DaggerAppComponent {
    fun repo(): DaggerRepo

    fun inject(target: FieldInjected)

    fun loginFactory(): DaggerLoginComponent.Factory

    @Component.Factory
    interface Factory {
        fun create(@BindsInstance name: String): DaggerAppComponent
    }
}

@Component
interface DaggerBuilderComponent {
    @Component.Builder
    interface Builder {
        @BindsInstance
        fun name(name: String): Builder

        fun build(): DaggerBuilderComponent
    }
}
