// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Guice scopes are not reported: keyed by full type.
// The declarations live here because the stubs have no framework layer.
package com.google.inject

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class ScopeAnnotation

@ScopeAnnotation
annotation class Singleton

@Singleton
class GuiceCache<T>

@Singleton
class GuiceService
