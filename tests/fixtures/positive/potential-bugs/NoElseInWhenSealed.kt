package fixtures.positive.potentialbugs

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

enum class Color { RED, GREEN, BLUE }

class NoElseInWhenSealed {
    // Missing variant on sealed type, no else.
    fun renderResult(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        is Result.Success -> r.value
        // missing: Failure
    }

    // Missing variant on sealed type, statement form.
    fun handle(r: Result) {
        when (r) {
            is Result.Loading -> println("loading")
            is Result.Success -> println(r.value)
            // missing: Failure
        }
    }

    // Missing entry on enum, no else.
    fun describe(c: Color): String = when (c) {
        Color.RED -> "warm"
        Color.BLUE -> "cool"
        // missing: GREEN
    }

    // Bare entry names also detected.
    fun describeBare(c: Color): String = when (c) {
        Color.RED -> "warm"
        // missing: GREEN, BLUE
    }
}
