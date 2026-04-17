package complexity

class ComplexCondition {
    fun check(a: Boolean, b: Boolean, c: Boolean) {
        if (a && b || c) {
            println("simple")
        }
    }
}
