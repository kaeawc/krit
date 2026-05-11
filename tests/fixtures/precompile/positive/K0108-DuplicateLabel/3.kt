// EXPECTED-KOTLINC-ERROR: DUPLICATE_LABEL_IN_WHEN
// String literal labels: same constant string repeated.
fun classify(tag: String): Int {
    return when (tag) {
        "yes" -> 1
        "no" -> 0
        "yes" -> 2
        else -> -1
    }
}
