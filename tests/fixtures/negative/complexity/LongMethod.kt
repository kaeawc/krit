package complexity

class LongMethod {
    fun shortFunction(x: Int): String {
        val result = x * 2
        val label = "value"
        val formatted = "$label: $result"
        if (result > 10) {
            return formatted.uppercase()
        }
        return formatted
    }
}
