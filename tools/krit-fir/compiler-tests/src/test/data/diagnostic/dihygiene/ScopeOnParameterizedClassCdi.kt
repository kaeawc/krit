// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 23, 26
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

<!ScopeOnParameterizedClass!>@ApplicationScoped<!>
class AppBean<T>

<!ScopeOnParameterizedClass!>@RequestScoped<!>
class RequestBean<T>

<!ScopeOnParameterizedClass!>@SessionScoped<!>
class SessionBean<T>

@ApplicationScoped
class PlainBean
