package fixtures.negative.style

fun hasPositive(list: List<Int>): Boolean {
    return list.any { it > 0 }
}
