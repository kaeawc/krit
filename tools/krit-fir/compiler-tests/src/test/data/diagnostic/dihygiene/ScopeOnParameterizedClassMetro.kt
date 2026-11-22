// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Metro scopes are not reported: keyed by full type.
// The shared Metro stub declares Scope; the scopes below are local.
package dev.zacsweers.metro

@Scope
annotation class Singleton

@Singleton
class MetroCache<T>

@Singleton
class MetroService
