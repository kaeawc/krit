// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 28, 31
// CDI normal scopes are meta-annotated with NormalScope, not
// jakarta.inject.Scope. The CDI declarations live here because the stubs have
// no CDI layer.
package jakarta.enterprise.context

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class NormalScope

@NormalScope
annotation class ApplicationScoped

@NormalScope
annotation class RequestScoped

@NormalScope
annotation class SessionScoped

// The pseudo-scope: meta-annotated with jakarta.inject.Scope, but it creates a
// new instance for every injection point.
@jakarta.inject.Scope
annotation class Dependent

<!ScopeOnParameterizedClass!>@ApplicationScoped<!>
class AppBean<T>

<!ScopeOnParameterizedClass!>@RequestScoped<!>
class RequestBean<T>

<!ScopeOnParameterizedClass!>@SessionScoped<!>
class SessionBean<T>

@ApplicationScoped
class PlainBean

// Go is silent (Dependent is not in its list) and so is FIR: a dependent bean
// gets a new instance per injection point, so no instance is shared across
// type arguments. It is the one scope CDI allows on a parameterized bean.
@Dependent
class DependentBean<T>
