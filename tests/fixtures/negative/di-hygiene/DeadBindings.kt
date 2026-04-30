package dihygiene

annotation class Module
annotation class Provides
annotation class Binds
annotation class Inject
annotation class Component

interface ApiOne
interface ApiTwo
interface ApiThree

class ApiOneImpl : ApiOne
class ApiTwoImpl : ApiTwo
class ApiThreeImpl : ApiThree

class ConsumerOne @Inject constructor(
    private val one: ApiOne,
)

class ConsumerTwo {
    @Inject
    lateinit var two: ApiTwo
}

@Component
interface AppComponent {
    fun three(): ApiThree
}

@Module
abstract class AppModule {
    @Provides
    fun provideOne(): ApiOne = ApiOneImpl()

    @Binds
    abstract fun bindTwo(impl: ApiTwoImpl): ApiTwo

    @Provides
    fun provideThree(): ApiThree = ApiThreeImpl()
}
