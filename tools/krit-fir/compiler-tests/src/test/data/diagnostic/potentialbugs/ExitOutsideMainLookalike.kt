// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 38, 42, 48
// Negatives: local lookalikes that do not terminate the process. None of these
// calls is kotlin.system.exitProcess or java.lang.System.exit, so the message
// "Do not call exitProcess() or System.exit()" is not true of them.
// Go reports every call below except `Worker().exit(1)`: it matches any call
// named `exitProcess` regardless of what it resolves to, and any `exit` whose
// receiver is spelled `System`. FIR is correct because each call resolves to
// a declaration in this file.
package test

// Same-package top-level function (kotlin.system is not a default import).
fun exitProcess(code: Int) {
    println("pretend exit $code")
}

object System {
    fun exit(code: Int) {
        println("pretend System.exit $code")
    }
}

class Worker {
    fun exitProcess(code: Int) {
        println("worker $code")
    }

    fun exit(code: Int) {
        println("worker exit $code")
    }
}

fun samePackageFunction() {
    exitProcess(1)
}

fun localSystemObject() {
    System.exit(1)
}

fun memberNamedExitProcess(worker: Worker) {
    worker.exitProcess(1)
    Worker().exit(1)
}

fun functionTypedValue() {
    val exitProcess: (Int) -> Unit = { println(it) }
    exitProcess(1)
}
