// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test

fun makeService(): Any = object {
    // The anonymous object's body is not traversed by the declaration index.
    fun service(): String {
        class Context { fun getSystemService(name: String): Any = Any() }
        return Context().getSystemService("service") as String
    }
}
