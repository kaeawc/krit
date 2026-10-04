// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The javax flavor of CDI's pseudo-scope `@Dependent`, meta-annotated with
// javax.inject.Scope but creating a new instance for every injection point.
// The declaration lives here because the stubs have no CDI layer.
package javax.enterprise.context

@javax.inject.Scope
annotation class Dependent

// Go is silent (Dependent is not in its list) and so is FIR: no instance is
// shared across type arguments.
@Dependent
class DependentBean<T>
