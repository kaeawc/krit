// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go and FIR agree on: a logger, lookalike printStackTrace functions
// on types that are not Throwables, and a callable reference.
package test

import java.util.logging.Level
import java.util.logging.Logger

class Printer {
    fun printStackTrace() {}
}

fun printStackTrace() {}

object Diagnostics {
    fun printStackTrace(message: String) {
        println(message)
    }
}

class Service {
    private val logger = Logger.getLogger("Service")

    fun logged() {
        try {
            doWork()
        } catch (e: Exception) {
            logger.log(Level.SEVERE, "Operation failed", e)
        }
    }

    fun lookalikes(printer: Printer) {
        printer.printStackTrace()
        printStackTrace()
        Diagnostics.printStackTrace("state")
    }

    fun lookalikesInCatch(printer: Printer) {
        try {
            doWork()
        } catch (e: Exception) {
            printer.printStackTrace()
            printStackTrace()
            Diagnostics.printStackTrace(e.toString())
        }
    }

    // A callable reference is not a call, in Go or here.
    fun reference(errors: List<Throwable>) {
        errors.forEach(Throwable::printStackTrace)
    }

    private fun doWork() {}
}
