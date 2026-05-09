package fixable.style

fun positives(xs: List<Int>): List<Int> {
    return xs.filterNot { !it.isOdd() }
}

fun Int.isOdd(): Boolean = this % 2 == 1
