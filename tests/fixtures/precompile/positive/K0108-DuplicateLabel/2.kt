// EXPECTED-KOTLINC-ERROR: DUPLICATE_LABEL_IN_WHEN
// Same literal appears twice in a single comma-separated branch.
fun describe(n: Int): String {
    return when (n) {
        1, 2, 1 -> "one or two"
        else -> "other"
    }
}
