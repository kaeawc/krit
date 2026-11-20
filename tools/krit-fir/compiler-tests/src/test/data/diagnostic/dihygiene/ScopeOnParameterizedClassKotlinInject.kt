// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// kotlin-inject scopes are not reported: keyed by full type.
// The declarations live here because the stubs have no framework layer.
package me.tatarka.inject.annotations

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class Scope

@Scope
annotation class Singleton

@Singleton
class InjectCache<T>

@Singleton
class InjectService
