// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 19
package test

@javax.enterprise.context.NormalScope
annotation class Singleton

@jakarta.enterprise.context.NormalScope
annotation class ApplicationScoped

@Singleton
class SingletonState {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

@ApplicationScoped
class ApplicationState {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}
