// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test

// Classifiers inside enum class bodies are not indexed as lookalikes.
enum class E {
    A;
    class Context { fun getSystemService(name: String): Any = Any() }
    fun service(context: Context): String = context.getSystemService("service") as String
}
