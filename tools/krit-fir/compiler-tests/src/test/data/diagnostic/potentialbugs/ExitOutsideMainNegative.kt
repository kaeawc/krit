// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: calls whose nearest enclosing named function is `main` (any
// owner, receiver or signature, as Go matches by name), including calls inside
// lambdas, anonymous functions and local-class initializers within main; exit
// methods on other receivers; and mentions that are not calls.
package test

import kotlin.system.exitProcess

class Process {
    fun exit(code: Int) {}
}

fun otherReceiver(process: Process) {
    process.exit(1)
    Runtime.getRuntime().exit(1)
    val text = "exitProcess(1) and System.exit(1)"
    // exitProcess(1)
    println(text)
}

fun reference(codes: List<Int>) {
    // A callable reference is not a call; Go does not report it either.
    codes.forEach(::exitProcess)
}

fun main() {
    if (System.getenv("KRIT_EXIT") != null) System.exit(0)
    Thread { exitProcess(1) }.start()
    listOf(1).forEach(fun(code: Int) { System.exit(code) })
    class Local {
        init {
            exitProcess(2)
        }
    }
    Local()
    exitProcess(0)
}

object App {
    @JvmStatic
    fun main(args: Array<String>) {
        System.exit(args.size)
    }
}

class Launcher {
    fun main(code: Int) {
        java.lang.System.exit(code)
    }

    companion object {
        suspend fun main() {
            kotlin.system.exitProcess(0)
        }
    }
}

fun String.main() {
    exitProcess(length)
}
