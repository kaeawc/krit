// Negative: range and type tests are not literal labels; the rule skips
// them because deduplicating overlapping ranges requires resolver
// context.
fun bucket(n: Int): String {
    return when (n) {
        in 0..9 -> "single"
        in 10..99 -> "double"
        in 0..9 -> "single again"
        else -> "more"
    }
}

// Negative: interpolated string literals are dynamic; their runtime
// value differs even when the source text matches.
fun classify(tag: String, suffix: String): Int {
    return when (tag) {
        "yes-$suffix" -> 1
        "no-$suffix" -> 0
        "yes-$suffix" -> 2
        else -> -1
    }
}
