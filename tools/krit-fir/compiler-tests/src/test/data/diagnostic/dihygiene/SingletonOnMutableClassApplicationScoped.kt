// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package jakarta.enterprise.context

annotation class ApplicationScoped

@ApplicationScoped
class ApplicationCache {
    <!SingletonOnMutableClass!>var value = 0<!>
    fun bump() { value++ }
}

@ApplicationScoped
class ReadOnly { val values = listOf<String>() }
