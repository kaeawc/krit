package fixtures.negative.potentialbugs

class UnreachableCode {
    // Normal code before return
    fun normalReturn(x: Int): Int {
        println("reachable")
        return x
    }

    // Conditional return (code after if-return is reachable)
    fun conditionalReturn(x: Int): Int {
        if (x > 0) return x
        println("still reachable")
        return -x
    }

    // Conditional throw
    fun conditionalThrow(x: Int) {
        if (x < 0) throw IllegalArgumentException("negative")
        println("still reachable")
    }

    // Code in different branches
    fun differentBranches(x: Int): String {
        return if (x > 0) {
            "positive"
        } else {
            "non-positive"
        }
    }

    // TODO() as the only statement (no following code)
    fun todoOnly(): Nothing {
        TODO()
    }

    // error() as the only statement
    fun errorOnly(): Nothing {
        error("not implemented")
    }

    // Conditional TODO
    fun conditionalTodo(debug: Boolean) {
        if (debug) TODO()
        println("still reachable")
    }

    // When branches (each branch is its own scope)
    fun whenBranches(x: Int): String {
        return when {
            x > 0 -> "positive"
            x < 0 -> "negative"
            else -> "zero"
        }
    }

    // If without else — not exhaustive
    fun ifWithoutElse(x: Int) {
        if (x > 0) {
            return
        }
        println("reachable — no else branch")
    }

    // while(true) with break — not infinite
    fun loopWithBreak(items: List<String>) {
        while (true) {
            if (items.isEmpty()) break
            println("working")
        }
        println("reachable after break")
    }

    // return with expression value — tree-sitter splits into two siblings
    fun returnWithValue(x: Int): List<Int> {
        if (x > 0) {
            return emptyList()
        }
        return listOf(x)
    }

    // return with complex expression
    fun returnWithComplexValue(items: List<String>): String {
        return items.joinToString(", ")
    }

    // throw with expression value — tree-sitter splits into two siblings
    fun throwWithValue(x: Int) {
        if (x < 0) {
            throw IllegalArgumentException("negative: $x")
        }
    }

    // return with labeled return
    fun returnLabeled(items: List<Int>): List<Int> {
        return items.filter {
            return@filter it > 0
        }
    }

    // bare return in Unit function (no value)
    fun bareReturn(x: Int) {
        if (x > 0) {
            println("positive")
            return
        }
        println("non-positive")
    }

    // return with method call chain
    fun returnWithChain(): List<String> {
        return listOf("a", "b", "c").filter { it.length > 1 }.map { it.uppercase() }
    }

    // Multiple returns in branches
    fun multipleReturns(x: Int): Int {
        when {
            x > 0 -> return x
            x < 0 -> return -x
        }
        return 0
    }
}
