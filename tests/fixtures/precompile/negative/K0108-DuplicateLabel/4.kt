// Different-typed integer literals are distinct constants to kotlinc:
// `1` is Int, `1L` is Long, `1u` is UInt — not duplicates.
fun mixed(x: Any): String = when (x) {
    1 -> "int"
    1L -> "long"
    1u -> "uint"
    else -> "other"
}
