package fixtures.negative.style

fun safe(nullableStr: String?): String {
    return nullableStr?.orEmpty() ?: ""
}
