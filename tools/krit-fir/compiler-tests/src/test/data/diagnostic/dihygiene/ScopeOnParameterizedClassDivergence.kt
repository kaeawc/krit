// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Inject, Hilt and project scopes do not trigger this rule.
// Their generic bindings are not class-keyed shared instances.
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

// An inject scope next to the lookalike is also silent.
@SingletonHolder @Singleton
class LookalikeAndScope<T>

// Hilt scopes next to lookalikes are silent too.
@SingletonHolder @ActivityScoped
class Mixed<T>
