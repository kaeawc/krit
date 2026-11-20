// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 18, 24, 27
// Koin's @Singleton and @Single declare single-instance definitions without a scope
// meta-annotation. An explicit import resolves it from the shared Koin stub
// layer.
package test

import org.koin.core.annotation.Singleton
import org.koin.core.annotation.Single


<!ScopeOnParameterizedClass!>@Singleton<!>
class KoinCache<T>

@Singleton
class KoinService

<!ScopeOnParameterizedClass!>@Single<!>
class KoinSingleCache<T>

@Single
class KoinSingleService

<!ScopeOnParameterizedClass!>@[Singleton]<!>
class BracketedCache<T>

<!ScopeOnParameterizedClass!>@[Suppress("unused") Single]<!>
class MultipleAnnotationsCache<T>
