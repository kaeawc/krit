package fixable.style

fun classify(c: Boolean): Int {
    return when {
        c -> 1
        else -> 2
    }
}
