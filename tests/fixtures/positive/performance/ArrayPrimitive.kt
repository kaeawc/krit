package fixtures.positive.performance

fun createArray(): Array<Int> {
    val arr = Array<Int>(5) { it }
    return arr
}
