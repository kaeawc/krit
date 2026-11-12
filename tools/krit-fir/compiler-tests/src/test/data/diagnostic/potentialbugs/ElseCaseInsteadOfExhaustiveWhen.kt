// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 20, 28, 36, 45, 55, 70, 78
package test

sealed class Shape
class Circle : Shape()
class Square : Shape()

enum class Color { RED, GREEN }

fun sealedSubject(shape: Shape): String {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (shape) {
        is Circle -> "circle"
        is Square -> "square"
        else -> "unknown"
    }
}

fun enumSubject(color: Color): Int {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (color) {
        Color.RED -> 1
        Color.GREEN -> 2
        else -> 0
    }
}

fun booleanSubject(flag: Boolean): Int {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (flag) {
        true -> 1
        false -> 0
        else -> -1
    }
}

fun nullableWithNullBranch(shape: Shape?): String {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (shape) {
        is Circle -> "circle"
        is Square -> "square"
        null -> "none"
        else -> "unknown"
    }
}

fun statementWhen(color: Color) {
    <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (color) {
        Color.RED -> println("r")
        Color.GREEN -> println("g")
        else -> println("other")
    }
}

typealias Hue = Color

fun aliasedSubject(color: Hue): Int {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (color) {
        Color.RED -> 1
        Color.GREEN -> 2
        else -> 0
    }
}

// Go does not see through the nested sealed hierarchy: Node is covered by its two subtypes.
sealed interface Node
sealed interface Leaf : Node
object Left : Leaf
object Right : Leaf
class Branch : Node

fun nestedSealed(node: Node): Int {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (node) {
        is Leaf -> 1
        is Branch -> 2
        else -> 3
    }
}

fun subjectVariable(shape: Shape): String {
    return <!ElseCaseInsteadOfExhaustiveWhen!>when<!> (val s = shape) {
        is Circle -> "circle"
        is Square -> "square"
        else -> s.toString()
    }
}
