package fixtures.positive.potentialbugs

sealed class Result {
    object Loading : Result()
    data class Success(val value: String) : Result()
    data class Failure(val error: Throwable) : Result()
}

enum class Color { RED, GREEN, BLUE }

class NonExhaustiveWhen {
    // Expression body — when consumed as expression, missing Failure variant.
    fun renderResult(r: Result): String = when (r) {
        is Result.Loading -> "loading"
        is Result.Success -> r.value
        // missing: Failure
    }

    // Property initializer — expression context, missing GREEN entry.
    fun describe(c: Color): String {
        val label: String = when (c) {
            Color.RED -> "warm"
            Color.BLUE -> "cool"
            // missing: GREEN
        }
        return label
    }

    // `return when` — expression context, missing Failure.
    fun summarize(r: Result): String {
        return when (r) {
            is Result.Loading -> "loading"
            is Result.Success -> "ok"
            // missing: Failure
        }
    }

    // Boolean subject as expression body — missing `false`.
    fun toInt(b: Boolean): Int = when (b) {
        true -> 1
        // missing: false
    }

    // when as call argument — expression context, missing Failure.
    fun log(r: Result) {
        println(when (r) {
            is Result.Loading -> "loading"
            is Result.Success -> "ok"
            // missing: Failure
        })
    }
}
