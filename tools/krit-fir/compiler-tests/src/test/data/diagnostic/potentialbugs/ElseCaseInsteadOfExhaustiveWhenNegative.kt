// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

sealed class Shape
class Circle : Shape()
class Square : Shape()
class Triangle : Shape()

enum class Color { RED, GREEN, BLUE }

interface Plain
class PlainA : Plain
class PlainB : Plain

fun notCovered(shape: Shape): String {
    return when (shape) {
        is Circle -> "circle"
        is Square -> "square"
        else -> "unknown"
    }
}

fun enumNotCovered(color: Color): Int {
    return when (color) {
        Color.RED -> 1
        Color.GREEN -> 2
        else -> 0
    }
}

fun guardingElse(shape: Shape): String {
    return when (shape) {
        is Circle -> "circle"
        is Square -> "square"
        is Triangle -> "triangle"
        else -> throw IllegalStateException("Unexpected shape")
    }
}

fun guardingElseBlock(color: Color): Int {
    return when (color) {
        Color.RED -> 1
        Color.GREEN -> 2
        Color.BLUE -> 3
        else -> {
            println("unreachable")
            error("unexpected")
        }
    }
}

fun nullableWithoutNullBranch(shape: Shape?): String {
    return when (shape) {
        is Circle -> "circle"
        is Square -> "square"
        is Triangle -> "triangle"
        else -> "none"
    }
}

fun plainInterface(p: Plain): Int {
    return when (p) {
        is PlainA -> 1
        is PlainB -> 2
        else -> 3
    }
}

fun subjectless(shape: Shape): String {
    return when {
        shape is Circle -> "circle"
        shape is Square -> "square"
        shape is Triangle -> "triangle"
        else -> "unknown"
    }
}

fun intSubject(n: Int): String {
    return when (n) {
        1 -> "one"
        else -> "many"
    }
}

fun noElse(shape: Shape): String {
    return when (shape) {
        is Circle -> "circle"
        is Square -> "square"
        is Triangle -> "triangle"
    }
}

fun localLookalike(): Int {
    class Local
    return when (Local()) {
        is Local -> 1
        else -> 2
    }
}
