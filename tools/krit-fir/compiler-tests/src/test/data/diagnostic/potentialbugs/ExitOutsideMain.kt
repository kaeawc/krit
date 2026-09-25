// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives: kotlin.system.exitProcess and java.lang.System.exit called where
// the nearest enclosing named function is not `main`, or where there is no
// enclosing function at all. Every shape here is reported by Go as well.
package test

import kotlin.system.exitProcess

// Top-level property initializer: no enclosing function.
val topLevelExit: Int = if (System.getenv("KRIT_EXIT") != null) <!ExitOutsideMain!>exitProcess(3)<!> else 0

fun topLevel() {
    <!ExitOutsideMain!>exitProcess(1)<!>
}

fun systemExit() {
    <!ExitOutsideMain!>System.exit(1)<!>
}

fun qualifiedSystemExit() {
    <!ExitOutsideMain!>java.lang.System.exit(2)<!>
}

fun qualifiedExitProcess() {
    <!ExitOutsideMain!>kotlin.system.exitProcess(2)<!>
}

fun multiLineQualifier() {
    <!ExitOutsideMain!>java.lang.System<!>
        .exit(4)
}

fun inLambda(codes: List<Int>) {
    codes.forEach { <!ExitOutsideMain!>exitProcess(it)<!> }
}

fun inAnonymousFunction(codes: List<Int>) {
    codes.forEach(fun(code: Int) { <!ExitOutsideMain!>System.exit(code)<!> })
}

fun inDefaultArgument(code: Int = <!ExitOutsideMain!>exitProcess(5)<!>): Int = code

// Named `main`-like but not `main`.
fun mainLoop() {
    <!ExitOutsideMain!>exitProcess(0)<!>
}

class Service {
    init {
        if (System.getenv("KRIT_EXIT") != null) <!ExitOutsideMain!>System.exit(1)<!>
    }

    val exitGetter: Int
        get() = <!ExitOutsideMain!>exitProcess(6)<!>

    constructor(code: Int) {
        if (code < 0) <!ExitOutsideMain!>exitProcess(code)<!>
    }

    fun stop() {
        <!ExitOutsideMain!>exitProcess(0)<!>
    }

    companion object {
        fun shutdown() {
            <!ExitOutsideMain!>System.exit(0)<!>
        }
    }
}

object Terminator {
    fun terminate() {
        <!ExitOutsideMain!>exitProcess(9)<!>
    }
}

interface Stoppable {
    fun stop() {
        <!ExitOutsideMain!>exitProcess(7)<!>
    }
}

fun main() {
    // A named function declared inside main is its own function: Go's nearest
    // function_declaration is `inner`, not `main`.
    fun inner() {
        <!ExitOutsideMain!>exitProcess(1)<!>
    }
    inner()

    // A member of a local class or object expression inside main is not main.
    class Local {
        fun run() {
            <!ExitOutsideMain!>System.exit(1)<!>
        }
    }
    Local().run()
    val anonymous = object {
        fun run() {
            <!ExitOutsideMain!>exitProcess(1)<!>
        }
    }
    anonymous.run()
}
