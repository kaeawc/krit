// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
package test

fun runIf(enabled: Boolean) {
    if (enabled) {
        // The if-expression wrapper prevents traversal into this local declaration.
        class Context { fun getSystemService(name: String): Any = Any() }
        Context().getSystemService("service") as String
    }
}
