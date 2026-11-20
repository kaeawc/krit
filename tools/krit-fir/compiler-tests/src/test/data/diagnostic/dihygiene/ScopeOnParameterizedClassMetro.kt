// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Metro scopes are not reported: keyed by full type.
// The declarations live here because the stubs have no framework layer.
package dev.zacsweers.metro

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class Scope

@Scope
annotation class Singleton

@Singleton
class MetroCache<T>

@Singleton
class MetroService
