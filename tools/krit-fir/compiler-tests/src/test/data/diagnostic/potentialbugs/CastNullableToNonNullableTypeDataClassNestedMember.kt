// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

data class Screen(val context: Context) {
    fun service(): String = context.getSystemService("service") as String

    class Context {
        fun getSystemService(name: String): Any = Any()
    }
}
