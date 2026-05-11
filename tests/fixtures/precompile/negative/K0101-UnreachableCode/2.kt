// Negative: jump used as an expression value, not in statement position.
fun elvisReturn(x: Int?): Int {
    val y = x ?: return -1
    return y + 1
}

fun ifBranchReturn(b: Boolean, x: Int): Int {
    val y = if (b) return x else x + 1
    return y
}
