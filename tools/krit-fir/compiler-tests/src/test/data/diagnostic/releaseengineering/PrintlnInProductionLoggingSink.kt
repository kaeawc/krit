// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25
// Logging sinks print on purpose: a log override and a class named like a logger. A plain class
// still reports.
package test

abstract class Handler {
    abstract fun publish(message: String)
}

class ConsoleHandler : Handler() {
    override fun publish(message: String) {
        println(message)
    }
}

class DebugLogger {
    fun write(message: String) {
        println(message)
    }
}

class Plain {
    fun run(message: String) {
        <!PrintlnInProduction!>println(message)<!>
    }
}
