package fixtures.negative.potentialbugs

sealed class Shape
class Circle : Shape()
class Square : Shape()

class ElseCaseInsteadOfExhaustiveWhen {
    fun describe(shape: Shape): String {
        return when (shape) {
            is Circle -> "circle"
            is Square -> "square"
        }
    }

    fun conditionBased(shape: Shape): String {
        return when {
            shape is Circle -> "circle"
            shape is Square -> "square"
            else -> "unknown"
        }
    }

    fun defensiveThrow(shape: Shape): String {
        return when (shape) {
            is Circle -> "circle"
            is Square -> "square"
            else -> throw IllegalStateException("Unexpected shape")
        }
    }
}
