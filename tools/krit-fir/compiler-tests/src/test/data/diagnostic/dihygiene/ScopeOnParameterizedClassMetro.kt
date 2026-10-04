// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Metro scopes are meta-annotated with Metro's own Scope, which the Metro stub
// does not declare, so it is declared here with a project scope named like one
// of Go's.
package dev.zacsweers.metro

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class Scope

@Scope
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class MetroCache<T>

@Singleton
class MetroService
