// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 22, 27, 32, 38
// Go matches `@<ScopeName>` as text anywhere in the class's modifier list.
// These generic classes carry no DI scope, so the message ("@Singleton on
// generic class ... shares one instance") is false of them.
package test

import javax.inject.Named
import javax.inject.Singleton

annotation class SingletonHolder

annotation class UserScopedCache

// Go reports this because `@SingletonHolder` starts with `@Singleton`; FIR is
// correct to drop it because SingletonHolder is not a scope.
@SingletonHolder
class PrefixLookalike<T>

// Go reports this because `@UserScopedCache` starts with `@UserScoped`; FIR is
// correct to drop it because UserScopedCache is not a scope.
@UserScopedCache
class PrefixLookalikeUserScoped<T>

// Go reports this because the qualifier's string argument holds `@Singleton`;
// FIR is correct to drop it because the class has only a qualifier.
@Named("@Singleton")
class InStringArgument<T>

// Go reports this because the comment between the annotations holds
// `@Singleton`; FIR is correct to drop it because the class is not scoped.
@Deprecated("replaced")
// was @Singleton before the cache became per-screen
@Suppress("unused")
class InComment<T>

// A real scope next to the lookalike is still reported, like Go.
<!ScopeOnParameterizedClass!>@SingletonHolder @Singleton<!>
class LookalikeAndScope<T>
