// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package test

@dev.zacsweers.metro.Scope
annotation class Singleton

@Singleton
class ScopedState {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}
