// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 20
package example.di

@javax.inject.Scope
annotation class Singleton

@jakarta.inject.Scope
annotation class ApplicationScoped

// Both resolve a project scope with Go's recognized simple name.
@Singleton
class ProjectCache {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

@ApplicationScoped
class ApplicationCache {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

@javax.inject.Scope
annotation class PerRequest

// A real scope with an unrelated lifetime is outside this rule's scope.
@PerRequest
class RequestCache { var value = 0; fun bump() { value++ } }
