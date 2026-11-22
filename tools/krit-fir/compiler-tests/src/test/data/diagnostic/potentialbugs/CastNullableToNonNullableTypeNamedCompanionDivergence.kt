// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7
package test

// Named companion objects are not indexed as lookalikes.
class Screen(private val context: Context) {
    fun service(): String = context.getSystemService("service") as String
    companion object Context { fun getSystemService(name: String): Any = Any() }
}
