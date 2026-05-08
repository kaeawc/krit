package fixtures.positive.potentialbugs

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
