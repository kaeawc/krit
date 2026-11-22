// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package test

class Host {
    val value: String
        get() {
            // Accessor bodies are not declaration-index traversal nodes.
            class Context { fun getSystemService(name: String): Any = Any() }
            return Context().getSystemService("service") as String
        }
}
