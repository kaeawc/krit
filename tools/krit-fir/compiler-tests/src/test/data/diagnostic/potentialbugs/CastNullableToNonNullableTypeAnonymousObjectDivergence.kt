// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
package test

fun makeService(): Any = object {
    // The anonymous object's body is not traversed by the declaration index.
    class Context { fun getSystemService(name: String): Any = Any() }
    fun service(context: Context): String = context.getSystemService("service") as String
}
