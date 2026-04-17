package fixtures.digraph

import kotlin.reflect.KClass

annotation class Component(val modules: Array<KClass<*>> = [])
annotation class Module
annotation class Provides
annotation class Inject

class DiskDao

class Api @Inject constructor()

class UserCache @Inject constructor(
    private val api: Api,
    private val diskDao: DiskDao,
)

@Module
object StorageModule {
    @Provides
    fun provideDiskDao(): DiskDao = DiskDao()
}

@Component(modules = [StorageModule::class])
interface AppComponent {
    fun cache(): UserCache
}
