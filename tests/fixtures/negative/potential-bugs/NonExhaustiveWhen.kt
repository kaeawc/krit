package fixtures.negative.potentialbugs

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

enum class Color { RED, GREEN, BLUE }

class NonExhaustiveWhen {
    // Statement form — handled by NoElseInWhenSealed, not this rule.
    fun handle(r: Result) {
        when (r) {
            is Result.Loading -> println("loading")
            is Result.Success -> println(r.value)
            // missing: Failure — but this is a statement, not an expression
        }
    }

    // Expression context, all sealed variants covered.
    fun render(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        is Result.Success -> r.value
        is Result.Failure -> r.error.message ?: "error"
    }

    // Expression context, all enum entries covered.
    fun describe(c: Color): String = when (c) {
        Color.RED -> "warm"
        Color.GREEN -> "fresh"
        Color.BLUE -> "cool"
    }

    // Expression context with else — else makes it exhaustive (out of scope).
    fun describeWithElse(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        else -> "ready"
    }

    // when{} without subject — out of scope.
    fun classify(x: Int): String = when {
        x > 0 -> "positive"
        x < 0 -> "negative"
        else -> "zero"
    }

    // Subject type isn't sealed/enum/Boolean — rule is silent.
    fun describeAny(x: Any): String = when (x) {
        is Int -> "int"
        is String -> "string"
        else -> "other"
    }

    // Boolean subject as expression body, both branches covered.
    fun toIntFull(b: Boolean): Int = when (b) {
        true -> 1
        false -> 0
    }

    // Boolean subject in statement context — not flagged.
    fun handleBool(b: Boolean) {
        when (b) {
            true -> println("yes")
            // not flagged: statement form
        }
    }
}
