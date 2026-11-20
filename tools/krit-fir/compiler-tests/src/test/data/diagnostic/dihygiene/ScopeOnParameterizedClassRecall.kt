// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 46, 49
// Inject, Hilt and project scopes do not trigger this rule.
// Their generic bindings are not class-keyed shared instances.
package test

import org.koin.core.annotation.Singleton as KoinSingleton
import javax.inject.Scope
import javax.inject.Singleton as AppSingleton

typealias Single = javax.inject.Singleton

@Scope
annotation class PerActivity

@Scope
annotation class UserScopedSession

// Inject scopes are silent even when qualified, aliased or project-defined.
@javax.inject.Singleton
class QualifiedScope<T>

// An import alias does not change the framework semantics.
@AppSingleton
class AliasedScope<T>

// A type alias does not change the framework semantics.
@Single
class TypeAliasedScope<T>

// Custom inject scopes are also silent.
@PerActivity
class ProjectScope<T>

// Custom inject scopes are silent on a generic fun interface too.
@PerActivity
fun interface Listener<T> {
    fun onEvent(event: T)
}

// A differently named project inject scope is also silent.
@UserScopedSession
class PrefixedProjectScope<T>

// Koin remains reported through qualified references and import aliases.
<!ScopeOnParameterizedClass!>@org.koin.core.annotation.Singleton<!>
class KoinQualified<T>

<!ScopeOnParameterizedClass!>@KoinSingleton<!>
class KoinAliased<T>

// FIR resolves the Koin type alias; Go deliberately leaves it unresolved.
typealias KoinSingle = org.koin.core.annotation.Singleton

<!ScopeOnParameterizedClass!>@KoinSingle<!>
class KoinTypeAliased<T>
