// RENDER_DIAGNOSTICS_FULL_TEXT
// Go reports `println("x")` here: the file declares no *function* named
// println, so Go takes the call for the built-in. It invokes the local val
// `println`, a lambda that only records its argument, and prints nothing to
// the console, so FIR does not report it.
package test

fun localLambda(sink: MutableList<String>) {
    val println = { s: String -> sink.add(s) }
    println("x")
}
