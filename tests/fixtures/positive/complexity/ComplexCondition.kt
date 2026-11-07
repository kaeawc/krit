package complexity

class ComplexCondition {
    fun check(a: Boolean, b: Boolean, c: Boolean, d: Boolean, e: Boolean) {
        if (a && b || c && d && e) {
            println("complex")
        }
    }

    fun checkWithLambda(items: List<Int>, a: Boolean) {
        if (a && items.any { it > 0 && it < 10 && it != 5 && it != 7 || it == 42 }) {
            println("detekt counts the operators in the lambda text too")
        }
    }
}
