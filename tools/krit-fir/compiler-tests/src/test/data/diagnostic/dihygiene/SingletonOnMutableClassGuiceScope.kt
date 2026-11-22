// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17
package com.google.inject

@Target(AnnotationTarget.ANNOTATION_CLASS)
annotation class ScopeAnnotation

@ScopeAnnotation
annotation class Singleton

// Go recognizes ApplicationScoped by name; FIR reaches it through ScopeAnnotation.
@ScopeAnnotation
annotation class ApplicationScoped

@ApplicationScoped
class ScopedState {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}
