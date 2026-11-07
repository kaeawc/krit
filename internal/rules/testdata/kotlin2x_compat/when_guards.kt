// rules: NoElseInWhenSealed, NonExhaustiveWhen, ElseCaseInsteadOfExhaustiveWhen, UseIfInsteadOfWhen
package compat

sealed interface Shape
class Circle(val radius: Int) : Shape
class Square : Shape
enum class Mode { Fast, Slow }

// A guarded branch matches only some instances, so it does not make the else redundant.
fun guardKeepsElse(shape: Shape, big: Boolean) = when (shape) {
    is Circle if big -> 1
    is Square -> 2
    else -> 3
}

// An unguarded branch for the same type does cover it, so the else is redundant.
fun unguardedCovers(shape: Shape, big: Boolean) = when (shape) { // expect: ElseCaseInsteadOfExhaustiveWhen
    is Circle if big -> 1
    is Circle -> 2
    is Square -> 3
    else -> 4
}

// Without an else the guarded branch leaves Circle uncovered.
fun guardLeavesGap(shape: Shape, big: Boolean) = when (shape) { // expect: NoElseInWhenSealed, NonExhaustiveWhen
    is Circle if big -> 1
    is Square -> 2
}

fun guardLeavesGapStatement(shape: Shape, big: Boolean) {
    when (shape) { // expect: NoElseInWhenSealed
        is Circle if big -> println()
        is Square -> println()
    }
}

fun guardedEnum(mode: Mode, strict: Boolean) = when (mode) {
    Mode.Fast if strict -> 1
    Mode.Slow -> 2
    else -> 3
}

fun guardedEnumCovered(mode: Mode, strict: Boolean) = when (mode) { // expect: ElseCaseInsteadOfExhaustiveWhen
    Mode.Fast if strict -> 1
    Mode.Fast -> 4
    Mode.Slow -> 2
    else -> 3
}

fun guardedBoolean(flag: Boolean, strict: Boolean) = when (flag) {
    true if strict -> 1
    false -> 2
    else -> 3
}

// detekt's shape: two entries and an else, no entry with several conditions.
fun binary(x: Int) = when (x) { // expect: UseIfInsteadOfWhen
    1 -> "one"
    else -> "other"
}

fun guardedBinary(x: Int, strict: Boolean) = when (x) {
    1 if strict -> "one"
    else -> "other"
}
