package fixtures.positive.potentialbugs

class MissingReturn {
    // Empty body, non-Unit return type.
    fun emptyBody(): Int {
    }

    // Body with statement, no return.
    fun fallThrough(x: Int): Int {
        println("got $x")
    }

    // when used as a statement without exhaustive coverage and without return on every branch.
    fun whenWithoutElse(x: Int): String {
        when (x) {
            1 -> "one"
            2 -> "two"
        }
    }

    // if without else.
    fun ifWithoutElse(x: Int): Int {
        if (x > 0) {
            return x
        }
    }

    // if with else but only one branch returns.
    fun ifWithPartialReturn(x: Int): Int {
        if (x > 0) {
            return x
        } else {
            println("negative")
        }
    }
}
