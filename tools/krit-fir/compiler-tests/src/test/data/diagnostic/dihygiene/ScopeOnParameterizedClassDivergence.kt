// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36, 41
// Generic classes whose modifier text mentions a scope name without carrying
// the scope. They carry no DI scope, so the message ("@Singleton on generic
// class ... shares one instance") is false of them, and neither Go nor FIR
// reports them.
package test

import dagger.hilt.android.scopes.ActivityScoped
import javax.inject.Named
import javax.inject.Singleton

annotation class SingletonHolder

annotation class UserScopedCache

// `@SingletonHolder` only starts with `@Singleton`.
@SingletonHolder
class PrefixLookalike<T>

// `@UserScopedCache` only starts with `@UserScoped`.
@UserScopedCache
class PrefixLookalikeUserScoped<T>

// The qualifier's string argument holds `@Singleton`.
@Named("@Singleton")
class InStringArgument<T>

// The comment between the annotations holds `@Singleton`.
@Deprecated("replaced")
// was @Singleton before the cache became per-screen
@Suppress("unused")
class InComment<T>

// A real scope next to the lookalike is still reported, like Go.
<!ScopeOnParameterizedClass!>@SingletonHolder<!> @Singleton
class LookalikeAndScope<T>

// Both report this line and name `@ActivityScoped`, the scope the class
// carries, not the `@SingletonHolder` lookalike.
<!ScopeOnParameterizedClass!>@SingletonHolder<!> @ActivityScoped
class Mixed<T>
