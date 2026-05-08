package complexity

class ComplexCondition {
    fun check(a: Boolean, b: Boolean, c: Boolean) {
        if (a && b || c) {
            println("simple")
        }
    }

    fun checkWithLambda(items: List<Int>, a: Boolean, b: Boolean) {
        if (a && items.any { it > 0 && it < 10 && it != 5 && it != 7 || it == 42 }) {
            println("lambda operators must not inflate outer condition")
        }
    }

    fun checkWithNestedFun(a: Boolean, b: Boolean) {
        if (a || b) {
            fun nested(x: Int, y: Int, z: Int): Boolean {
                return x > 0 && y > 0 && z > 0 && x < y && y < z
            }
            println(nested(1, 2, 3))
        }
    }
}
