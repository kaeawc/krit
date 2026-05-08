package fixtures.negative.performance

fun createArray(): IntArray {
    val arr = IntArray(5) { it }
    return arr
}
