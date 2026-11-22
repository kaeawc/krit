// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

class Screen(private val context: Context) {
    fun service(): String = context.getSystemService("service") as String
    class Context {
        fun getSystemService(name: String): Any = Any()
    }
}
