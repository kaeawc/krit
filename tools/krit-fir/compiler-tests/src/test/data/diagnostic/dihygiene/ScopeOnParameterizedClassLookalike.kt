// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 30
// Local lookalikes: annotations in this package named like DI scopes but not
// meta-annotated as scopes, so no DI framework treats them as one.
package test

import javax.inject.Scope

annotation class Singleton

annotation class ActivityScoped

// Neither reports this: test.Singleton, declared here without a scope
// meta-annotation and not imported from elsewhere, is not a DI scope.
@Singleton
class LocalSingleton<T>

// Neither reports this: test.ActivityScoped is not a DI scope either.
@ActivityScoped
class LocalActivityScoped<T>

// A project scope named like one of Go's scopes is a real scope, like Go.
@Scope
annotation class UserScoped

<!ScopeOnParameterizedClass!>@UserScoped<!>
class ProjectScoped<T>

// A qualified reference still reaches the real scope, like Go.
<!ScopeOnParameterizedClass!>@javax.inject.Singleton<!>
class RealSingleton<T>
