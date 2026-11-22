// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
package test

fun makeService(): () -> String = fun(): String {
    // Anonymous-function bodies are not declaration-index traversal nodes.
    class Context { fun getSystemService(name: String): Any = Any() }
    return Context().getSystemService("service") as String
}
