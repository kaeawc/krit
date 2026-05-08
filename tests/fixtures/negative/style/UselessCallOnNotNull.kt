package fixtures.negative.style

fun safe(nullableStr: String?): String {
    return nullableStr?.orEmpty() ?: ""
}

// Navigation with nullable chain — safe call means receiver might be null
fun safeNavigation(obj: String?): String {
    return obj?.orEmpty() ?: ""
}
