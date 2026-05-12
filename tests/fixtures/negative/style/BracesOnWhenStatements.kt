package style

fun example(x: Int): String {
    return when (x) {
        1 -> {
            "one"
        }
        2 -> {
            "two"
        }
        else -> {
            "other"
        }
    }
}

// Expression-position whens must not be flagged — wrapping their branch
// bodies in braces would transform the assignment into a visually-broken
// block (the `}` aligns to the outer statement's indent, not the when's).
fun expressionPosition(x: Int): String {
    val a = when (x) {
        1 -> "one"
        else -> "other"
    }
    val b: String = when (x) {
        1 -> "one"
        else -> "other"
    }
    return a + b + when (x) {
        1 -> "one"
        else -> "other"
    }
}

fun argument(x: Int) {
    println(when (x) {
        1 -> "one"
        else -> "other"
    })
}
