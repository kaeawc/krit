// Negative: each literal label is unique.
fun describe(n: Int): String {
    return when (n) {
        1 -> "one"
        2 -> "two"
        3 -> "three"
        else -> "other"
    }
}

// Negative: same literal across two different when expressions is fine —
// scope is per-when.
fun two(n: Int, m: Int): Int {
    val a = when (n) {
        1 -> 10
        else -> 0
    }
    val b = when (m) {
        1 -> 20
        else -> 0
    }
    return a + b
}
