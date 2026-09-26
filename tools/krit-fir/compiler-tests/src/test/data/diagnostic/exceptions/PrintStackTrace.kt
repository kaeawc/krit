// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 27, 35, 44, 53, 62, 72, 83, 91, 92, 105, 120
// printStackTrace() on the caught exception, the shapes Go reports: the
// receiver is the nearest enclosing catch's variable. The finding sits on the
// call expression's first line, its receiver's, like Go.
package test

import java.io.IOException
import java.io.PrintWriter
import java.io.StringWriter

class CustomFailure(message: String) : RuntimeException(message)

class Service {
    fun process() {
        try {
            doWork()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace()<!>
        }
    }

    fun safeCall() {
        try {
            doWork()
        } catch (t: Throwable) {
            <!PrintStackTrace!>t?.printStackTrace()<!>
        }
    }

    fun toStream() {
        try {
            doWork()
        } catch (e: IOException) {
            <!PrintStackTrace!>e.printStackTrace(System.err)<!>
        }
    }

    fun toWriter(): String {
        val writer = StringWriter()
        try {
            doWork()
        } catch (e: CustomFailure) {
            <!PrintStackTrace!>e.printStackTrace(PrintWriter(writer))<!>
        }
        return writer.toString()
    }

    fun splitAcrossLines() {
        try {
            doWork()
        } catch (e: Exception) {
            <!PrintStackTrace!>e<!>
                .printStackTrace()
        }
    }

    fun insideLambda(tasks: List<Runnable>) {
        try {
            doWork()
        } catch (e: Exception) {
            tasks.forEach { _ -> <!PrintStackTrace!>e.printStackTrace()<!> }
        }
    }

    fun insideObject(): Runnable {
        try {
            doWork()
        } catch (e: Exception) {
            return object : Runnable {
                override fun run() {
                    <!PrintStackTrace!>e.printStackTrace()<!>
                }
            }
        }
        return Runnable {}
    }

    fun asExpression(): Int = try {
        doWork()
        1
    } catch (e: Exception) {
        <!PrintStackTrace!>e.printStackTrace()<!>
        0
    }

    fun twice() {
        try {
            doWork()
        } catch (e: Exception) {
            <!PrintStackTrace!>e.printStackTrace()<!>
            <!PrintStackTrace!>e.printStackTrace(System.out)<!>
        }
    }

    private fun doWork() {
        throw CustomFailure("failure")
    }
}

fun topLevel() {
    try {
        error("boom")
    } catch (ex: IllegalStateException) {
        <!PrintStackTrace!>ex.printStackTrace()<!>
    }
}

// A custom exception that overrides printStackTrace() is still a Throwable.
class LoudFailure : Exception() {
    override fun printStackTrace() {
        println("loud")
    }
}

fun overridden(failure: () -> Unit) {
    try {
        failure()
    } catch (e: LoudFailure) {
        <!PrintStackTrace!>e.printStackTrace()<!>
    }
}
