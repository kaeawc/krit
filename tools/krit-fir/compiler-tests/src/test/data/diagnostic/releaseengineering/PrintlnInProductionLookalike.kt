// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 41, 42
// Negative: calls named println / print that do not print to the console.
package test

// A same-file declaration shadows the built-in (Go also skips these: the
// file declares a function named println).
fun println(label: String): String = label

class Recorder {
    fun print(label: String): String = label

    fun emit() {
        val recorded = print("member shadows kotlin.io.print")
        require(recorded.isNotEmpty())
    }
}

fun usesShadowed() {
    val recorded = println("top-level shadows kotlin.io.println")
    require(recorded.isNotEmpty())
    fun print(label: String): String = label
    require(print("local shadows kotlin.io.print").isNotEmpty())
}

// Go matches `System.out` by spelling and reports these two calls; the
// receiver is this file's System object, not java.lang.System, so nothing is
// printed to the console and FIR does not report them.
object System {
    object out {
        fun println(message: String) {}
    }
    val err: Sink = Sink()
}

class Sink {
    fun print(message: String) {}
}

fun lookalikeSystem() {
    System.out.println("not the console")
    System.err.print("not the console")
}

// Go stops at a longer chain or another member name, and so does FIR.
fun longerChains() {
    java.lang.System.out.checkError()
}
