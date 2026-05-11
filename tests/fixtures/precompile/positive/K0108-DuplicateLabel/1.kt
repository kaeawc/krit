// EXPECTED-KOTLINC-ERROR: DUPLICATE_LABEL_IN_WHEN
fun describeInt(n: Int): String {
    return when (n) {
        1 -> "one"
        2 -> "two"
        1 -> "also one"
        else -> "other"
    }
}
