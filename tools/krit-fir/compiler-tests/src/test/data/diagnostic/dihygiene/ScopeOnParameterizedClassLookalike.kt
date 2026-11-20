// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Inject, Hilt and project scopes do not trigger this rule.
// Their generic bindings are not class-keyed shared instances.
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

// A project inject scope is not reported.
@Scope
annotation class UserScoped

@UserScoped
class ProjectScoped<T>

// A qualified inject scope is not reported either.
@javax.inject.Singleton
class RealSingleton<T>
