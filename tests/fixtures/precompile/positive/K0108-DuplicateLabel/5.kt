// EXPECTED-KOTLINC-ERROR: DUPLICATE_LABEL_IN_WHEN
// kotlinc dedupes integer constants regardless of base or separators:
// `0x01`, `1_000`, and `1000` are all the same Int value.
fun classify(n: Int): String = when (n) {
    1 -> "one"
    0x01 -> "hex one"
    1_000 -> "thousand"
    1000 -> "also thousand"
    else -> "other"
}
