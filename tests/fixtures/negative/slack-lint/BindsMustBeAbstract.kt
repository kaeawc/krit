package fixtures.negative.slacklint

import dagger.Binds
import dagger.Module

interface Repository
class RealRepository : Repository

@Module
abstract class RepositoryModule {
    @Binds abstract fun bindRepository(impl: RealRepository): Repository
}

@Module
interface InterfaceModule {
    @Binds fun bindRepository(impl: RealRepository): Repository
}
