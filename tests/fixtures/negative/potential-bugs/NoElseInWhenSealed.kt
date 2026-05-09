package fixtures.negative.potentialbugs

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

enum class Color { RED, GREEN, BLUE }

class NoElseInWhenSealed {
    // All sealed variants covered — no missing branches.
    fun renderResult(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        is Result.Success -> r.value
        is Result.Failure -> r.error.message ?: "error"
    }

    // Has an else branch — out of scope (handled by ElseCaseInsteadOfExhaustiveWhen).
    fun renderWithElse(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        else -> "ready"
    }

    // All enum entries covered — no missing branches.
    fun describe(c: Color): String = when (c) {
        Color.RED -> "warm"
        Color.GREEN -> "fresh"
        Color.BLUE -> "cool"
    }

    // when{} without subject — not exhaustiveness-checked.
    fun classify(x: Int): String = when {
        x > 0 -> "positive"
        x < 0 -> "negative"
        else -> "zero"
    }

    // Subject type is not sealed/enum — the rule cannot determine variants.
    fun describeAny(x: Any): String = when (x) {
        is Int -> "int"
        is String -> "string"
        else -> "other"
    }

    // Plain Int — not sealed/enum.
    fun classify2(x: Int): String = when (x) {
        1 -> "one"
        2 -> "two"
        else -> "many"
    }
}
