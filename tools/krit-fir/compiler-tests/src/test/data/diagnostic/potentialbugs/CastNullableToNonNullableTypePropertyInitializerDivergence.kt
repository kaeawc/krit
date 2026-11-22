// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test

class Host {
    val value: String = run {
        // The property initializer's call expression hides the local class from indexing.
        class Context { fun getSystemService(name: String): Any = Any() }
        Context().getSystemService("service") as String
    }
}
