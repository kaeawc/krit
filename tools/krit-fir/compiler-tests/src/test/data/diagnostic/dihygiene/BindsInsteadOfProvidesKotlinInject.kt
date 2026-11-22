// go-lines: none
package bindsinstead.kotlininject

import dagger.Module
import me.tatarka.inject.annotations.Provides

interface InjectService
class InjectImplementation : InjectService

@Module
abstract class InjectModule {
    @Provides fun kotlinInject(implementation: InjectImplementation): InjectService = implementation
    @dagger.Provides fun daggerAlongsideInject(implementation: InjectImplementation): InjectService = implementation
}
