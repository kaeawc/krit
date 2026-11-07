// rules: ElseCaseInsteadOfExhaustiveWhen, EnumNaming
package compat

enum class Color { RED, GREEN, BLUE }
enum class Mixed { lower, Upper, UPPER_CASE } // expect: EnumNaming

fun allValues() = Color.values()
fun allEntries() = Color.entries
fun firstEntry() = Color.entries.first()

fun describe(color: Color) = when (color) {
    Color.RED -> "r"
    Color.GREEN -> "g"
    Color.BLUE -> "b"
}

fun describeWithElse(color: Color) = when (color) { // expect: ElseCaseInsteadOfExhaustiveWhen
    Color.RED -> "r"
    Color.GREEN -> "g"
    Color.BLUE -> "b"
    else -> "x"
}

fun partial(color: Color) = when (color) {
    Color.RED -> "r"
    else -> "other"
}
