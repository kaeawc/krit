package performance

fun createArrays() {
    val ints = IntArray(10) { it * 2 }
    val bools = BooleanArray(5) { it % 2 == 0 }
}
