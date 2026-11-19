// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 16, 18, 23, 28, 29, 34
// Go reports every bare print / println, including one that resolves to a
// member of an implicit PrintStream / PrintWriter receiver. Only a receiver
// that is System.out / System.err prints to the console; FIR reports those
// and not the calls that write to another stream.
package test

import java.io.PrintStream
import java.io.PrintWriter
import java.io.StringWriter

fun console() {
    with(System.out) { <!PrintlnInProduction!>println<!>("console") }
    System.err.run { <!PrintlnInProduction!>print<!>("console") }
    System.out.apply { <!PrintlnInProduction!>println<!>("console") }
    // A receiver FIR does not trace to System.err (a call result) is not reported.
    java.lang.System.err.also { it.flush() }.apply { println("chained") }
}

fun intoString(): String {
    val sw = StringWriter()
    PrintWriter(sw).apply { println("into a string") }
    return sw.toString()
}

fun intoStream(stream: PrintStream) {
    with(stream) { println("into a caller stream") }
    stream.run { print("into a caller stream") }
}

class Indenting(out: StringWriter) : PrintWriter(out) {
    fun line(text: String) {
        println("  $text")
    }
}
