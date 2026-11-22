// go-lines: none
package bindsinstead.kotlininjectwildcard

import dagger.Module
import dagger.Provides
import me.tatarka.inject.annotations.*

interface WildcardService
class WildcardImplementation : WildcardService

@Module
abstract class WildcardModule {
    // Both engines preserve the kotlin-inject import exemption, including wildcard imports.
    @Provides fun provide(implementation: WildcardImplementation): WildcardService = implementation
}
