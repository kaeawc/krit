package fixtures.positive.potentialbugs

private fun failHard(msg: String): Nothing = throw IllegalStateException(msg)

class UnreachableCode {
    // Statement after return
    fun afterReturn(x: Int): Int {
        return x
        println("unreachable")
    }

    // Statement after throw
    fun afterThrow() {
        throw IllegalStateException("boom")
        println("unreachable")
    }

    // Statement after break
    fun afterBreak() {
        for (i in 1..10) {
            break
            println("unreachable")
        }
    }

    // Statement after continue
    fun afterContinue() {
        for (i in 1..10) {
            continue
            println("unreachable")
        }
    }

    // Statement after TODO()
    fun afterTodo() {
        TODO()
        println("unreachable")
    }

    // Statement after error()
    fun afterError() {
        error("fatal")
        println("unreachable")
    }

    // Statement after exitProcess()
    fun afterExitProcess() {
        exitProcess(0)
        println("unreachable")
    }

    // Statement after fully-qualified kotlin.system.exitProcess()
    fun afterQualifiedExitProcess() {
        kotlin.system.exitProcess(0)
        println("unreachable")
    }

    // Statement after a workspace-defined Nothing-returning function.
    // The resolver picks up failHard() via r.functions and reports its
    // return type as Nothing, so the rule treats it as a jump.
    fun afterWorkspaceNothing() {
        failHard("boom")
        println("unreachable")
    }

    // Code after if where all branches return
    fun afterExhaustiveIf(x: Int): String {
        if (x > 0) {
            return "positive"
        } else {
            return "non-positive"
        }
        println("unreachable")
    }

    // Code after while(true) with no break
    fun afterInfiniteLoop() {
        while (true) {
            println("forever")
        }
        println("unreachable")
    }
}
