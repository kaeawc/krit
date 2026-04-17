package complexity

class ComplexCondition {
    fun check(a: Boolean, b: Boolean, c: Boolean, d: Boolean, e: Boolean) {
        if (a && b || c && d && e) {
            println("complex")
        }
    }
}
