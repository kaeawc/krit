// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
// kotlin-inject scopes are meta-annotated with kotlin-inject's own Scope. The
// declarations live here because the stubs have no kotlin-inject layer.
package me.tatarka.inject.annotations

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class Scope

@Scope
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class InjectCache<T>

@Singleton
class InjectService
