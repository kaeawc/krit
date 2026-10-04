// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 20, 27
// Local lookalikes: annotations in this package named like DI scopes but not
// meta-annotated as scopes, so no DI framework treats them as one.
package test

import javax.inject.Scope

annotation class Singleton

annotation class ActivityScoped

// Go reports this because the annotation is named Singleton; FIR is correct to
// drop it because test.Singleton is not a DI scope.
@Singleton
class LocalSingleton<T>

// Go reports this because the annotation is named ActivityScoped; FIR is
// correct to drop it because test.ActivityScoped is not a DI scope.
@ActivityScoped
class LocalActivityScoped<T>

// A project scope named like one of Go's scopes is a real scope, like Go.
@Scope
annotation class UserScoped

<!ScopeOnParameterizedClass!>@UserScoped<!>
class ProjectScoped<T>

// A qualified reference still reaches the real scope; Go misses it because it
// matches only the unqualified `@Singleton`.
<!ScopeOnParameterizedClass!>@javax.inject.Singleton<!>
class RealSingleton<T>
