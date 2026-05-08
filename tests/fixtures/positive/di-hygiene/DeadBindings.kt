package dihygiene

annotation class Module
annotation class Provides
annotation class Inject

interface UsedApi
interface DeadApi

class UsedApiImpl : UsedApi
class DeadApiImpl : DeadApi

class Consumer @Inject constructor(
    private val used: UsedApi,
)

@Module
object AppModule {
    @Provides
    fun provideUsed(): UsedApi = UsedApiImpl()

    @Provides
    fun provideDead(): DeadApi = DeadApiImpl()
}
