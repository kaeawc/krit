// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 28
// Console output on purpose: a function named like a print verb, and one whose KDoc names System.out or
// System.err. Names that only contain "print" still report.
package test

/** Records reads and prints to [System.out] whenever state changes. */
fun observe(id: String) {
    printChange(id)
}

private fun printChange(id: String) {
    println("$id changed")
}

/** Writes the line to System.err for the developer. */
fun report(line: String) {
    System.err.println(line)
}

class Tracer {
    fun print(line: String) {
        println(line)
    }
}

fun printer(line: String) {
    <!PrintlnInProduction!>println(line)<!>
}
