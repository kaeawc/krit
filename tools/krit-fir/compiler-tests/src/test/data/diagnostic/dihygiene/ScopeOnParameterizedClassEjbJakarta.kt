// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// The Jakarta EE flavor of EJB's @Singleton session bean, which carries no
// scope meta-annotation. The declaration lives here because the stubs have no
// EJB layer.
package jakarta.ejb

@Target(AnnotationTarget.CLASS)
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class SingletonBean<T>

@Singleton
class PlainSingletonBean
