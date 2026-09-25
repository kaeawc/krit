// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: console output the Go rule exempts, and print-like calls that are
// not console output.
package test

annotation class TaskAction

// The top-level entry point may print, including inside its lambdas and the
// bodies of classes it declares.
fun main(args: Array<String>) {
    println("script entry point")
    args.forEach { println(it) }
    class Local {
        init {
            println("local class init")
        }
    }
    Local()
    System.out.println("done")
}

// Gradle task actions print to the build log. Go matches the annotation by its
// simple name, in any package.
abstract class GenerateTask {
    @TaskAction
    fun execute() {
        println("generated")
        listOf(1).forEach { System.err.println(it) }
    }
}

class Output {
    val out: Output get() = this
    fun println(message: String) {}
    fun print(message: String) {}
}

fun receivers(out: Output, builder: StringBuilder) {
    // Only System.out / System.err receivers count.
    out.println("done")
    out.out.println("done")
    builder.append("x")
    val stream = System.out
    stream.println("through a local")
    // Other PrintStream methods are not print / println.
    System.out.printf("%s", "x")
    System.out.write(1)
    System.out.flush()
    // A callable reference is not a call.
    listOf("a").forEach(::println)
}
