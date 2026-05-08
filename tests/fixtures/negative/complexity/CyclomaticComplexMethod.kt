package complexity

class CyclomaticComplexMethod {
    fun simpleFunction(x: Int): String {
        if (x > 0) {
            return "positive"
        } else if (x < 0) {
            return "negative"
        }
        return "zero"
    }
}
