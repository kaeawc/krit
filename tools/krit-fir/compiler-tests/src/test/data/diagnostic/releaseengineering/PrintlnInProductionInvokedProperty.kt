// RENDER_DIAGNOSTICS_FULL_TEXT
// Go reports `println("x")` here: the file declares no *function* named
// println, so Go takes the call for the built-in. It invokes the `println`
// property, a caller-supplied lambda, and prints nothing to the console, so
// FIR does not report it.
package test

class Formatter(private val println: (String) -> Unit) {
    fun emit() {
        println("x")
    }
}
