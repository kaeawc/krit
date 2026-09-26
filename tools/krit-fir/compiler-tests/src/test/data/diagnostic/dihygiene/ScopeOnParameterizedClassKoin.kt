// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// Koin's @Singleton declares a single-instance definition without a scope
// meta-annotation. The declaration lives here because the stubs have no Koin
// layer.
package org.koin.core.annotation

@Target(AnnotationTarget.CLASS, AnnotationTarget.FUNCTION)
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class KoinCache<T>

@Singleton
class KoinService
