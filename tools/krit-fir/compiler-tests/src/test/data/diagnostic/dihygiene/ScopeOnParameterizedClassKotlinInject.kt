// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// kotlin-inject scopes are not reported: keyed by full type.
// The shared kotlin-inject stub declares Scope; the scopes below are local.
package me.tatarka.inject.annotations

@Scope
annotation class Singleton

@Singleton
class InjectCache<T>

@Singleton
class InjectService
