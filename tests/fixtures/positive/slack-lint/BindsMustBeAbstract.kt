package fixtures.positive.slacklint

import dagger.Binds
import dagger.Module

interface Repository
class RealRepository : Repository

@Module
abstract class RepositoryModule {
    @Binds fun bindRepository(impl: RealRepository): Repository = impl
}

@Module
interface InterfaceModule {
    @Binds fun bindRepository(impl: RealRepository): Repository {
        return impl
    }
}
