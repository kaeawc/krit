// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 22, 25, 28
// JSF managed-bean scopes carry no scope meta-annotation. The declarations
// live here because the stubs have no JSF layer.
package javax.faces.bean

@Target(AnnotationTarget.CLASS)
annotation class ApplicationScoped

@Target(AnnotationTarget.CLASS)
annotation class SessionScoped

@Target(AnnotationTarget.CLASS)
annotation class RequestScoped

@Target(AnnotationTarget.CLASS)
annotation class ViewScoped

<!ScopeOnParameterizedClass!>@ApplicationScoped<!>
class ApplicationBean<T>

<!ScopeOnParameterizedClass!>@SessionScoped<!>
class SessionBean<T>

<!ScopeOnParameterizedClass!>@RequestScoped<!>
class RequestBean<T>

<!ScopeOnParameterizedClass!>@ViewScoped<!>
class ViewBean<T>

@ApplicationScoped
class PlainBean
