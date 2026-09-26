// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 44
// Scoped generic types Go misses because it matches only the unqualified
// names in its scope list: each is a DI scope on a generic class, so the
// message is true of it.
package test

import javax.inject.Scope
import javax.inject.Singleton as AppSingleton

typealias Single = javax.inject.Singleton

@Scope
annotation class PerActivity

@Scope
annotation class UserScopedSession

// Go misses this because the scope is written with its package.
<!ScopeOnParameterizedClass!>@javax.inject.Singleton<!>
class QualifiedScope<T>

// Go misses this because the scope is imported under an alias.
<!ScopeOnParameterizedClass!>@AppSingleton<!>
class AliasedScope<T>

// Go misses this because the scope is reached through a type alias.
<!ScopeOnParameterizedClass!>@Single<!>
class TypeAliasedScope<T>

// Go misses this because PerActivity is not in its scope-name list.
<!ScopeOnParameterizedClass!>@PerActivity<!>
class ProjectScope<T>

// Go misses this because tree-sitter does not parse a `fun interface` as a
// class declaration; Go reports the same scope on a plain interface.
<!ScopeOnParameterizedClass!>@PerActivity<!>
fun interface Listener<T> {
    fun onEvent(event: T)
}

// Go reports this too (as `@UserScoped`, the list name the text starts with);
// the message here names the annotation itself.
<!ScopeOnParameterizedClass!>@UserScopedSession<!>
class PrefixedProjectScope<T>
