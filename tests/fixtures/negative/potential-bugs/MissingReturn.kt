package fixtures.negative.potentialbugs

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
}

class MissingReturn {
    // Implicit Unit return — no issue.
    fun unit() {
        println("ok")
    }

    // Explicit Unit return — no issue.
    fun explicitUnit(): Unit {
        println("ok")
    }

    // Returns Nothing — no issue (Nothing-typed functions don't return normally).
    fun crash(): Nothing {
        throw IllegalStateException("boom")
    }

    // Last statement is a return.
    fun normalReturn(x: Int): Int {
        return x + 1
    }

    // Last statement is a throw.
    fun alwaysThrow(): Int {
        throw IllegalStateException("not implemented")
    }

    // Last statement is TODO() (Nothing-returning).
    fun pendingImpl(): String {
        TODO("not yet")
    }

    // Expression body — implicitly returns its expression.
    fun expr(x: Int): Int = x + 1

    // Expression body via when — exhaustive.
    fun classify(x: Int): String = when {
        x > 0 -> "positive"
        x < 0 -> "negative"
        else -> "zero"
    }

    // if with both branches returning.
    fun branchesReturn(x: Int): Int {
        if (x > 0) {
            return x
        } else {
            return -x
        }
    }

    // when with else, all branches return.
    fun handle(x: Int): Int {
        when (x) {
            0 -> return 0
            else -> return -x
        }
    }

    // when on sealed type, all variants return.
    fun render(r: Result): String {
        when (r) {
            is Result.Loading -> return "loading"
            is Result.Success -> return r.value
        }
    }

    // Abstract — no body, no issue.
    abstract class Holder {
        abstract fun get(): Int
    }

    // Interface — no body, no issue.
    interface Provider {
        fun fetch(): String
    }
}
