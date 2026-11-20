// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import org.koin.core.annotation.*
import jakarta.enterprise.context.NormalScope

// FIR resolves the wildcard-imported Koin scope; Go leaves wildcard imports unresolved.
<!ScopeOnParameterizedClass!>@Singleton<!>
class WildcardCache<T>

// FIR resolves the wildcard-imported Koin scope; Go leaves wildcard imports unresolved.
<!ScopeOnParameterizedClass!>@Single<!>
class WildcardSingleCache<T>

@NormalScope
annotation class Tenant

// FIR follows custom CDI @NormalScope annotations; Go does not resolve custom scopes.
<!ScopeOnParameterizedClass!>@Tenant<!>
class TenantCache<T>
