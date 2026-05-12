// Negative: subject-less `when` uses arbitrary boolean conditions, not
// labels. The rule deliberately skips this shape because the conditions
// require resolver context to deduplicate safely.
fun describeBool(n: Int): String {
    return when {
        n > 0 -> "positive"
        n > 0 -> "still positive"
        else -> "non-positive"
    }
}

// Negative: identifier labels (enum entries, qualified references) need
// resolution to know whether two names refer to the same constant, so
// the rule skips them.
enum class Color { RED, GREEN, BLUE }

fun describe(c: Color): String {
    return when (c) {
        Color.RED -> "r"
        Color.GREEN -> "g"
        Color.BLUE -> "b"
    }
}
