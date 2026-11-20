// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package test

@me.tatarka.inject.annotations.Scope
annotation class Singleton

@Singleton
class ScopedState {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}
