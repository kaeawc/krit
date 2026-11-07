// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 41
// Logging sinks print on purpose: a log override, a class named like a logger, a Tree subclass and
// an anonymous Tree. A plain class still reports.
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
