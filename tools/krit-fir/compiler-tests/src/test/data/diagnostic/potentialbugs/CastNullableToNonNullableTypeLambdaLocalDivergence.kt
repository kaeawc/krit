// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package test

fun runLambda() {
    run {
        // The local class is behind a call expression, outside the declaration index traversal.
        class Context { fun getSystemService(name: String): Any = Any() }
        val context = Context()
        context.getSystemService("service") as String
    }
}
