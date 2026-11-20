// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 27, 30, 42
// CDI normal scopes are meta-annotated with NormalScope, not
// jakarta.inject.Scope. The shared NormalScope stub is used below.
package jakarta.enterprise.context

@NormalScope
annotation class ApplicationScoped

@NormalScope
annotation class RequestScoped

@NormalScope
annotation class SessionScoped

@NormalScope
annotation class ConversationScoped

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

<!ScopeOnParameterizedClass!>@ConversationScoped<!>
class ConversationBean<T>

@javax.inject.Singleton
class JavaxInjectBean<T>

@jakarta.inject.Singleton
class JakartaInjectBean<T>
