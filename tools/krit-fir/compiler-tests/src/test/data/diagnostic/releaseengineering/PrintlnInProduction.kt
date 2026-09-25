// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 11, 12, 17, 22, 29, 35, 40, 41, 42, 43, 44, 48, 51, 58, 59, 67, 72, 78, 84, 92, 95, 100, 105
// Positive: console output in production code, the shapes the Go rule
// reports. The finding sits on the first line of the call, as Go reports the
// start of the call_expression.
package test

class UserService {
    fun load() {
        <!PrintlnInProduction!>println("loading")<!>
        <!PrintlnInProduction!>print("loading")<!>
        <!PrintlnInProduction!>println()<!>
    }

    // A member function named main is not the top-level entry point.
    fun main() {
        <!PrintlnInProduction!>println("member main")<!>
    }

    companion object {
        fun log() {
            <!PrintlnInProduction!>println("companion")<!>
        }
    }
}

object Registry {
    fun log(value: Int) {
        <!PrintlnInProduction!>println(value)<!>
    }
}

interface Reporter {
    fun report() {
        <!PrintlnInProduction!>println("default method")<!>
    }
}

fun topLevel() {
    <!PrintlnInProduction!>System.out.println("out")<!>
    <!PrintlnInProduction!>System.err.println("err")<!>
    <!PrintlnInProduction!>System.out.print("out")<!>
    <!PrintlnInProduction!>System.err.print('c')<!>
    <!PrintlnInProduction!>System.out?.println("safe call")<!>
}

fun multiLine() {
    <!PrintlnInProduction!>System<!>
        .out
        .println("hi")
    <!PrintlnInProduction!>System<!>
        .err
        .println("hi")
}

// Lambdas do not end the search for the enclosing function.
fun inLambda(items: List<Int>) {
    items.forEach { <!PrintlnInProduction!>println(it)<!> }
    val action = { <!PrintlnInProduction!>print("later")<!> }
    action()
}

// A bare call through an implicit JDK print-stream receiver is still a bare
// println, which Go reports.
fun withStream() {
    with(System.out) {
        <!PrintlnInProduction!>println("implicit receiver")<!>
    }
}

fun withWriter(writer: java.io.PrintWriter) {
    writer.apply { <!PrintlnInProduction!>print("implicit writer")<!> }
}

// A local function is the nearest enclosing function, even inside main.
fun main() {
    fun helper() {
        <!PrintlnInProduction!>println("local helper")<!>
    }
    helper()
    // The nearest named function of an object-expression member is that member.
    val task = object : Runnable {
        override fun run() {
            <!PrintlnInProduction!>println("run")<!>
        }
    }
    task.run()
}

// Extensions, property initializers and accessors are production code too.
fun String.shout() {
    <!PrintlnInProduction!>println(this)<!>
}

val banner = run { <!PrintlnInProduction!>println("banner")<!> }

class WithAccessor {
    val value: Int
        get() {
            <!PrintlnInProduction!>println("getter")<!>
            return 1
        }

    init {
        <!PrintlnInProduction!>println("init")<!>
    }
}
