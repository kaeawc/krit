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
}
