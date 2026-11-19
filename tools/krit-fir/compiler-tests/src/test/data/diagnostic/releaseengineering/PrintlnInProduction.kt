// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 17, 22, 29, 35, 40, 41, 42, 43, 44, 50, 53, 58, 59, 69, 74, 80, 86, 94, 97, 102, 107
// Positive: console output in production code, the shapes the Go rule
// reports. The finding sits on the called name, as Go anchors a call on its
// callee, so a multi-line chain reports on the `.println` line.
package test

class UserService {
    fun load() {
        <!PrintlnInProduction!>println<!>("loading")
        <!PrintlnInProduction!>print<!>("loading")
        <!PrintlnInProduction!>println<!>()
    }

    // A member function named main is not the top-level entry point.
    fun main() {
        <!PrintlnInProduction!>println<!>("member main")
    }

    companion object {
        fun log() {
            <!PrintlnInProduction!>println<!>("companion")
        }
    }
}

object Registry {
    fun log(value: Int) {
        <!PrintlnInProduction!>println<!>(value)
    }
}

interface Reporter {
    fun report() {
        <!PrintlnInProduction!>println<!>("default method")
    }
}

fun topLevel() {
    System.out.<!PrintlnInProduction!>println<!>("out")
    System.err.<!PrintlnInProduction!>println<!>("err")
    System.out.<!PrintlnInProduction!>print<!>("out")
    System.err.<!PrintlnInProduction!>print<!>('c')
    System.out?.<!PrintlnInProduction!>println<!>("safe call")
}

fun multiLine() {
    System
        .out
        .<!PrintlnInProduction!>println<!>("hi")
    System
        .err
        .<!PrintlnInProduction!>println<!>("hi")
}

// Lambdas do not end the search for the enclosing function.
fun inLambda(items: List<Int>) {
    items.forEach { <!PrintlnInProduction!>println<!>(it) }
    val action = { <!PrintlnInProduction!>print<!>("later") }
    action()
}

// A bare call through the implicit System.out receiver of a lambda is still
// console output, which Go reports as a bare println. Go also reports the
// call on a caller-supplied writer; it writes to that writer, not the
// console, so FIR does not report it.
fun withStream() {
    with(System.out) {
        <!PrintlnInProduction!>println<!>("implicit receiver")
    }
}

fun withWriter(writer: java.io.PrintWriter) {
    writer.apply { print("implicit writer") }
}

// A local function is the nearest enclosing function, even inside main.
fun main() {
    fun helper() {
        <!PrintlnInProduction!>println<!>("local helper")
    }
    helper()
    // The nearest named function of an object-expression member is that member.
    val task = object : Runnable {
        override fun run() {
            <!PrintlnInProduction!>println<!>("run")
        }
    }
    task.run()
}

// Extensions, property initializers and accessors are production code too.
fun String.shout() {
    <!PrintlnInProduction!>println<!>(this)
}

val banner = run { <!PrintlnInProduction!>println<!>("banner") }

class WithAccessor {
    val value: Int
        get() {
            <!PrintlnInProduction!>println<!>("getter")
            return 1
        }

    init {
        <!PrintlnInProduction!>println<!>("init")
    }
}
