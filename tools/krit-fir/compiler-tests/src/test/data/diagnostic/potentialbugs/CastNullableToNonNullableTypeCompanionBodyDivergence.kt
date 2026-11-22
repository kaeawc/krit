// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test

// Classifiers inside companion objects are not indexed as lookalikes.
class Screen {
    companion object {
        class Context { fun getSystemService(name: String): Any = Any() }
        fun service(context: Context): String = context.getSystemService("service") as String
    }
}
