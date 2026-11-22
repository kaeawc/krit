// go-lines: 27, 30
package bindsinstead.annotations

import dagger.Module
import dagger.Provides
import javax.inject.Named
import javax.inject.Singleton

interface AnnotationService
class AnnotationImplementation : AnnotationService

@Target(AnnotationTarget.FUNCTION, AnnotationTarget.VALUE_PARAMETER)
annotation class Shared

@Module
abstract class AnnotationModule {
    @Provides @Singleton
    fun scoped(implementation: AnnotationImplementation): AnnotationService = implementation

    @Provides @Named("output")
    fun qualifier(implementation: AnnotationImplementation): AnnotationService = implementation

    @Provides
    fun parameterQualifier(@Named("input") implementation: AnnotationImplementation): AnnotationService = implementation

    @Provides @Shared
    fun <!BindsInsteadOfProvides!>shared<!>(@Shared implementation: AnnotationImplementation): AnnotationService = implementation

    @Provides @Named("same")
    fun <!BindsInsteadOfProvides!>sharedQualifier<!>(@Named("same") implementation: AnnotationImplementation): AnnotationService = implementation

    // Go is silent: the fully-qualified provider name is not removed from its annotation-name set.
    @dagger.Provides fun qualified(implementation: AnnotationImplementation): AnnotationService = implementation

    // FIR-only: resolution proves a Dagger provider; Go misses the bracketed annotation syntax.
    @[Provides] fun <!BindsInsteadOfProvides!>bracketed<!>(implementation: AnnotationImplementation): AnnotationService = implementation
}
