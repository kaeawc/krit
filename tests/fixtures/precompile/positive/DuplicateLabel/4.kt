// EXPECTED-KOTLINC-ERROR: DUPLICATE_LABEL_IN_WHEN
// `null` is parsed as a non-named token child of when_condition.
fun nullable(x: Int?): String = when (x) {
    null -> "n1"
    1 -> "one"
    null -> "n2"
    else -> "other"
}
