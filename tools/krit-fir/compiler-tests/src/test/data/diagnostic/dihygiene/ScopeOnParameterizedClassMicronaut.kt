// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Micronaut's `@Prototype` is meta-annotated with jakarta.inject.Scope, but it
// creates a new instance for every injection point. The declaration lives here
// because the stubs have no Micronaut layer.
package io.micronaut.context.annotation

@jakarta.inject.Scope
annotation class Prototype

// Go is silent (Prototype is not in its list) and so is FIR: no instance is
// shared across type arguments.
@Prototype
class PrototypeBean<T>
