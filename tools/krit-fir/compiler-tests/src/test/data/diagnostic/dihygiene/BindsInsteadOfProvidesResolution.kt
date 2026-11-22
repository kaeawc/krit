// go-lines: 14, 22
package bindsinstead.resolution

import dagger.Module
import dagger.Provides

interface ResolutionService
class ResolutionImplementation : ResolutionService
typealias MaybeService = ResolutionService?

@Module
abstract class ResolutionModule {
    // Go misses the alias's nullability and reports; FIR resolves the nullable parameter and return type.
    @Provides fun nullableAlias(implementation: MaybeService): MaybeService = implementation

    // Go counts the function type's named parameter as another value parameter and is silent.
    // Resolution proves that the bare returned expression is the single value parameter itself.
    @Provides
    fun <!BindsInsteadOfProvides!>functionType<!>(callback: (input: ResolutionImplementation) -> ResolutionService) = callback

    @Provides
    fun <!BindsInsteadOfProvides!>`escaped name`<!>(`escaped parameter`: ResolutionImplementation): ResolutionService = `escaped parameter`
}
