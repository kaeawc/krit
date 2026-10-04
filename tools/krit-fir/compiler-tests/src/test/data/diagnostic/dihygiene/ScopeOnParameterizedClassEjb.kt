// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// EJB's @Singleton marks a singleton session bean (one instance per
// application) without a scope meta-annotation. The declaration lives here
// because the stubs have no EJB layer.
package javax.ejb

@Target(AnnotationTarget.CLASS)
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class SingletonBean<T>

@Singleton
class PlainSingletonBean
