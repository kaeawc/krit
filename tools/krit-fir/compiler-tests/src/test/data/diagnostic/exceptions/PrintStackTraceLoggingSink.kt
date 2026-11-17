// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 42, 56, 66, 75
// Logging sinks print on purpose: a log override, a class named like a logger, a Tree subclass and
// an anonymous Tree, so a receiver Go does not report is skipped there. A plain class still reports,
// and so does a caught exception anywhere.
package test

abstract class Tree {
    abstract fun log(priority: Int, message: String, t: Throwable?)
}

class ConsoleTree : Tree() {
    override fun log(priority: Int, message: String, t: Throwable?) {
        t?.printStackTrace()
    }
}

class FileLogger {
    fun handle(t: Throwable) {
        t.printStackTrace()
    }
}

class SinkHelper : Tree() {
    override fun log(priority: Int, message: String, t: Throwable?) {}
    fun other(t: Throwable) {
        t.printStackTrace()
    }
}

val anonymous = object : Tree() {
    override fun log(priority: Int, message: String, t: Throwable?) {
        t?.printStackTrace()
    }
}

class Plain {
    fun run() {
        try {
            work()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace()<!>
        }
    }

    private fun work() {}
}

// A caught exception is reported in a logging sink too, like Go and detekt:
// the sink exemption only covers receivers Go does not report.
class ReportLogger {
    fun flush(work: () -> Unit) {
        try {
            work()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace()<!>
        }
    }
}

class GuardedTree : Tree() {
    override fun log(priority: Int, message: String, t: Throwable?) {
        try {
            println(message)
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace()<!>
        }
    }
}

fun printSummary(work: () -> Unit) {
    try {
        work()
    } catch (e: Exception) {
        <!PrintStackTrace!>e.printStackTrace()<!>
    }
}
