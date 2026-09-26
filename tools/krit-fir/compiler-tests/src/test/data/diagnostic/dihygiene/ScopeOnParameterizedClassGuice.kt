// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Guice scopes are meta-annotated with Guice's own ScopeAnnotation, not
// javax.inject.Scope. The Guice declarations live here because the stubs have
// no Guice layer.
package com.google.inject

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class ScopeAnnotation

@ScopeAnnotation
annotation class Singleton

<!ScopeOnParameterizedClass!>@Singleton<!>
class GuiceCache<T>

@Singleton
class GuiceService
